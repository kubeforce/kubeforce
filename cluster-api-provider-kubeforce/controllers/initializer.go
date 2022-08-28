package controllers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &Initializer{}

// Initializer initializes default k8s resources
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
		return err
	}
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
	diff, err := patchObj.Data(repo)
	if err != nil {
		return errors.Wrapf(err, "failed to calculate patch data")
	}

	// Unmarshal patch data into a local map.
	patchDiff := map[string]interface{}{}
	if err := json.Unmarshal(diff, &patchDiff); err != nil {
		return errors.Wrapf(err, "failed to unmarshal patch data into a map")
	}

	if len(patchDiff) > 0 {
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
	r.Spec.URL = "https://github.com/kubeforce/plugins/releases/download/"
	r.Spec.Timeout = &metav1.Duration{Duration: 10 * time.Second}
}
