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
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/assets"
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
	utiltmpl "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/templates"
)

var _ manager.Runnable = &Initializer{}

// Initializer initializes default k8s resources.
type Initializer struct {
	Log    logr.Logger
	Client client.Client
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooktemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbookdeploymenttemplates,verbs=get;list;watch;create;update;patch;delete

// Start starts the Initializer controller.
func (i *Initializer) Start(ctx context.Context) error {
	id := "_init_"
	backOff := flowcontrol.NewBackOff(time.Second, 5*time.Minute)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
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
	if err := i.ensurePlaybookTemplate(ctx, assets.PlaybookInstaller); err != nil {
		return errors.Wrapf(err, "unable to initialize PlaybookTemplate %s", assets.PlaybookInstaller)
	}
	if err := i.ensurePlaybookTemplate(ctx, assets.PlaybookCleaner); err != nil {
		return errors.Wrapf(err, "unable to initialize PlaybookTemplate %s", assets.PlaybookCleaner)
	}
	if err := i.ensurePlaybookDeploymentTemplate(ctx, assets.PlaybookLoadbalancer); err != nil {
		return errors.Wrapf(err, "unable to initialize PlaybookDeploymentTemplate %s", assets.PlaybookLoadbalancer)
	}
	i.Log.Info("all default k8s resources have been initialized")
	return nil
}

const (
	githubRepoName    = "github"
	kfSystemNamespace = "kubeforce-system"
)

func (i *Initializer) ensureGitHubRepo(ctx context.Context) error {
	repo := &infrav1.HTTPRepository{}
	key := client.ObjectKey{
		Namespace: kfSystemNamespace,
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
	r.Namespace = kfSystemNamespace
	r.Spec.URL = "https://github.com/kubeforce/kubeforce/releases/download/"
	if r.Spec.Timeout == nil {
		r.Spec.Timeout = &metav1.Duration{Duration: 90 * time.Second}
	}
}

func (i *Initializer) ensurePlaybookTemplate(ctx context.Context, assetName assets.PlaybookName) error {
	tmpl := &infrav1.PlaybookTemplate{}
	key := client.ObjectKey{
		Namespace: kfSystemNamespace,
		Name:      utiltmpl.GetName(assetName),
	}
	err := i.Client.Get(ctx, key, tmpl)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	created := true
	if apierrors.IsNotFound(err) {
		created = false
	}
	patchObj := client.MergeFrom(tmpl.DeepCopy())
	data, err := assets.GetPlaybook(assetName, nil)
	if err != nil {
		return err
	}
	tmpl.Name = key.Name
	tmpl.Namespace = key.Namespace
	tmpl.Spec.Spec.Entrypoint = data.Entrypoint
	tmpl.Spec.Spec.Files = data.Files
	if !created {
		i.Log.Info("creating PlaybookTemplate", "key", key)
		err := i.Client.Create(ctx, tmpl)
		if err != nil {
			return err
		}
		return nil
	}

	changed, err := patchutil.HasChanges(patchObj, tmpl)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		i.Log.Info("updating PlaybookTemplate", "key", key)
		err := i.Client.Patch(ctx, tmpl, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch PlaybookTemplate")
		}
		return nil
	}
	return nil
}

func (i *Initializer) ensurePlaybookDeploymentTemplate(ctx context.Context, assetName assets.PlaybookName) error {
	tmpl := &infrav1.PlaybookDeploymentTemplate{}
	key := client.ObjectKey{
		Namespace: kfSystemNamespace,
		Name:      utiltmpl.GetName(assetName),
	}
	err := i.Client.Get(ctx, key, tmpl)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	created := true
	if apierrors.IsNotFound(err) {
		created = false
	}
	patchObj := client.MergeFrom(tmpl.DeepCopy())
	data, err := assets.GetPlaybook(assetName, nil)
	if err != nil {
		return err
	}
	tmpl.Name = key.Name
	tmpl.Namespace = key.Namespace
	tmpl.Spec.Template.Spec.Entrypoint = data.Entrypoint
	tmpl.Spec.Template.Spec.Files = data.Files
	if !created {
		i.Log.Info("creating PlaybookDeploymentTemplate", "key", key)
		err := i.Client.Create(ctx, tmpl)
		if err != nil {
			return err
		}
		return nil
	}

	changed, err := patchutil.HasChanges(patchObj, tmpl)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		i.Log.Info("updating PlaybookDeploymentTemplate", "key", key)
		err := i.Client.Patch(ctx, tmpl, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch PlaybookDeploymentTemplate")
		}
		return nil
	}
	return nil
}
