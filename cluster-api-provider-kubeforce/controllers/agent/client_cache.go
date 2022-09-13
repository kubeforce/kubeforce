/*
Copyright 2022 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	agentclient "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	agentScheme "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned/scheme"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type ClientCache struct {
	log                   logr.Logger
	clientUncachedObjects []client.Object
	client                client.Client

	lock          sync.RWMutex
	clientHolders map[client.ObjectKey]*clientHolder
}

// NewClientCache creates a new ClientCache.
func NewClientCache(manager ctrl.Manager) (*ClientCache, error) {
	return &ClientCache{
		log:           manager.GetLogger(),
		client:        manager.GetClient(),
		clientHolders: make(map[client.ObjectKey]*clientHolder),
	}, nil
}

type clientHolder struct {
	checksum  string
	clientSet *agentclient.Clientset
	cache     *stoppableCache
	client    client.Client
	watches   sets.String
}

// GetClientSet returns a cached client for the given agent.
func (c *ClientCache) GetClientSet(ctx context.Context, agent client.ObjectKey) (*agentclient.Clientset, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	holder, err := c.getClientHolder(ctx, agent)
	if err != nil {
		return nil, err
	}
	if holder == nil {
		return nil, nil
	}

	return holder.clientSet, nil
}

// getClientHolder first tries to return an already-created clientHolder for agent, falling back to creating a
// new clientHolder if needed. Note, this method requires t.lock to already be held.
func (c *ClientCache) getClientHolder(ctx context.Context, agent client.ObjectKey) (*clientHolder, error) {
	h := c.clientHolders[agent]
	if h != nil {
		return h, nil
	}

	h, err := c.newClientHolder(ctx, agent)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating client and cache for agent %q", agent)
	}

	c.clientHolders[agent] = h

	return h, nil
}

// DeleteHolder stops a clientHolder's cache and removes the clientHolder.
func (c *ClientCache) DeleteHolder(agentKey client.ObjectKey) {
	c.lock.Lock()
	defer c.lock.Unlock()

	a, exists := c.clientHolders[agentKey]
	if !exists {
		return
	}

	c.log.Info("Deleting clientHolder", "agent", agentKey.String())

	c.log.Info("Stopping cache", "agent", agentKey.String())
	a.cache.Stop()
	c.log.Info("Cache stopped", "agent", agentKey.String())

	delete(c.clientHolders, agentKey)
}

func (c *ClientCache) getChecksum(agentKey client.ObjectKey) string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	h, exists := c.clientHolders[agentKey]
	if !exists {
		return ""
	}
	return h.checksum
}

func (c *ClientCache) calcChecksum(host string, keys *agent.Keys) (string, error) {
	jsonData, err := json.Marshal(keys)
	if err != nil {
		return "", errors.WithStack(err)
	}
	h := sha256.New()
	_, err = h.Write(jsonData)
	if err != nil {
		return "", errors.WithStack(err)
	}
	_, err = h.Write([]byte(host))
	if err != nil {
		return "", errors.WithStack(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *ClientCache) newClientHolder(ctx context.Context, agentKey client.ObjectKey) (*clientHolder, error) {
	kfAgent := &infrav1.KubeforceAgent{}
	if err := c.client.Get(ctx, agentKey, kfAgent); err != nil {
		return nil, errors.Wrapf(err, "unable to get agent %q", agentKey)
	}

	agentKeys, err := agent.GetKeys(ctx, c.client, kfAgent)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get agent keys %q", agentKey)
	}
	host, err := agent.GetServer(*kfAgent.Spec.Addresses)
	if err != nil {
		return nil, err
	}
	sha256sum, err := c.calcChecksum(host, agentKeys)
	if err != nil {
		return nil, err
	}

	restConfig := agent.NewClientConfig(agentKeys, host)
	clientset, err := agentclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// Create a mapper for it
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating dynamic rest mapper for agent %q", agentKey)
	}

	// Create the client for the remote cluster
	ctrlClient, err := client.New(restConfig, client.Options{Scheme: agentScheme.Scheme, Mapper: mapper})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating client for agent %q", agentKey)
	}

	// Create the cache for the remote cluster
	cacheOptions := cache.Options{
		Scheme: agentScheme.Scheme,
		Mapper: mapper,
	}
	remoteCache, err := cache.New(restConfig, cacheOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating cache for agent %q", agentKey)
	}

	cacheCtx, cacheCtxCancel := context.WithCancel(ctx)

	// We need to be able to stop the cache's shared informers, so wrap this in a stoppableCache.
	cache := &stoppableCache{
		Cache:      remoteCache,
		cancelFunc: cacheCtxCancel,
	}

	// Start the cache!!!
	go cache.Start(cacheCtx) //nolint:errcheck
	if !cache.WaitForCacheSync(cacheCtx) {
		return nil, errors.Wrapf(err, "failed waiting for cache for agent %q", agentKey)
	}

	delegatingClient, err := client.NewDelegatingClient(client.NewDelegatingClientInput{
		CacheReader: cache,
		Client:      ctrlClient,
	})
	if err != nil {
		return nil, err
	}

	return &clientHolder{
		checksum:  sha256sum,
		clientSet: clientset,
		cache:     cache,
		client:    delegatingClient,
		watches:   sets.NewString(),
	}, nil
}
