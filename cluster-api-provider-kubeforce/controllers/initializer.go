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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
)

var _ manager.Runnable = &Initializer{}

// Initializer initializes default k8s resources.
type Initializer struct {
	Log    logr.Logger
	Client client.Client
}

func (i *Initializer) Start(ctx context.Context) error {
	id := "_init_"
	backOff := flowcontrol.NewBackOff(time.Second, 5*time.Minute)
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := i.init(ctx)
		if err == nil {
			return nil
		}
		i.Log.Error(err, "unable to initialize default k8s resources")
		now := backOff.Clock.Now()
		backOff.Next(id, now)
		duration := backOff.Get(id)
		time.Sleep(duration)
	}
}

func (i *Initializer) init(ctx context.Context) error {
	if err := i.ensureGitHubRepo(ctx); err != nil {
		return errors.Wrapf(err, "unable to initialize 'github' HTTPRepository")
	}
	i.Log.Info("all default k8s resources have been initialized")
	return nil
}

const (
	githubRepoName      = "github"
	githubRepoNamespace = "kubeforce-system"
)

func (i *Initializer) ensureGitHubRepo(ctx context.Context) error {
	repo := &infrav1.HTTPRepository{}
	key := client.ObjectKey{
		Namespace: githubRepoNamespace,
		Name:      githubRepoName,
	}
	err := i.Client.Get(ctx, key, repo)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	created := true
	if apierrors.IsNotFound(err) {
		created = false
	}
	patchObj := client.MergeFrom(repo.DeepCopy())
	i.defaultGitHubRepo(repo)
	if !created {
		i.Log.Info("creating GitHubRepo", "key", key)
		err := i.Client.Create(ctx, repo)
		if err != nil {
			return err
		}
		return nil
	}

	changed, err := patchutil.HasChanges(patchObj, repo)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		i.Log.Info("updating GitHubRepo", "key", key)
		err := i.Client.Patch(ctx, repo, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch HTTPRepository")
		}
		return nil
	}
	return nil
}

func (i *Initializer) defaultGitHubRepo(r *infrav1.HTTPRepository) {
	r.Name = githubRepoName
	r.Namespace = githubRepoNamespace
	r.Spec.URL = "https://github.com/kubeforce/kubeforce/releases/download/"
	if r.Spec.Timeout == nil {
		r.Spec.Timeout = &metav1.Duration{Duration: 90 * time.Second}
	}
}
