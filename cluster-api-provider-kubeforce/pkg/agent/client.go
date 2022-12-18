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
	"net"
	"time"

	"github.com/pkg/errors"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
)

// NewClientConfigByAddress creates a new rest.Config from Keys and infrav1.Addresses.
func NewClientConfigByAddress(keys *Keys, addresses infrav1.Addresses) (*restclient.Config, error) {
	host, err := GetServer(addresses)
	if err != nil {
		return nil, err
	}
	return NewClientConfig(keys, host), nil
}

// GetServer returns http url for address.
func GetServer(addresses infrav1.Addresses) (string, error) {
	address := stringutil.Find(stringutil.IsNotEmpty, addresses.ExternalDNS, addresses.ExternalIP)
	if address == "" {
		return "", errors.Errorf("not found external address")
	}
	server := "https://" + net.JoinHostPort(address, "5443")
	return server, nil
}

// NewClientConfig creates a rest.Config.
func NewClientConfig(keys *Keys, host string) *restclient.Config {
	config := restclient.Config{
		QPS:     restclient.DefaultQPS,
		Burst:   restclient.DefaultBurst,
		Timeout: 10 * time.Second,
		Host:    host,
		TLSClientConfig: restclient.TLSClientConfig{
			CertData:   keys.AuthClient.Cert,
			KeyData:    keys.AuthClient.Key,
			CAData:     keys.TLS.CA,
			NextProtos: []string{"h2"},
		},
	}
	if len(config.TLSClientConfig.CAData) == 0 {
		config.TLSClientConfig.CAData = keys.TLS.Cert
	}
	return &config
}

// NewClientKubeconfig creates a kubeconfig for agent.
func NewClientKubeconfig(keys *Keys, server string) api.Config {
	clusterName := "kubernetes"
	contextName := "default"
	username := "agent"
	clusters := make(map[string]*api.Cluster)
	clusters[clusterName] = &api.Cluster{
		Server:                   server,
		CertificateAuthorityData: keys.TLS.CA,
	}

	contexts := make(map[string]*api.Context)
	contexts[contextName] = &api.Context{
		Cluster:  clusterName,
		AuthInfo: username,
	}

	authinfos := make(map[string]*api.AuthInfo)
	authinfos[username] = &api.AuthInfo{
		ClientCertificateData: keys.AuthClient.Cert,
		ClientKeyData:         keys.AuthClient.Key,
	}

	apiConfig := api.Config{
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: contextName,
		AuthInfos:      authinfos,
	}
	return apiConfig
}

// NewClientSet creates a new Clientset.
func NewClientSet(keys *Keys, addresses infrav1.Addresses) (*clientset.Clientset, error) {
	config, err := NewClientConfigByAddress(keys, addresses)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(config)
}

// IsHealthy returns true if the agent is ready to connect.
func IsHealthy(a *infrav1.KubeforceAgent) bool {
	return a.Spec.Installed && conditions.IsTrue(a, infrav1.HealthyCondition)
}

// IsReady returns true if the agent is ready, all operational states are in order.
func IsReady(a *infrav1.KubeforceAgent) bool {
	return a.Spec.Installed && conditions.IsTrue(a, infrav1.HealthyCondition) && conditions.IsTrue(a, clusterv1.ReadyCondition)
}
