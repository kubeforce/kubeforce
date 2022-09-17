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

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/repository"
)

// HTTPRepositoryReconciler is responsible for removing cached files from storage when
// the HTTPRepository is being deleted.
type HTTPRepositoryReconciler struct {
	Client  client.Client
	Storage *repository.Storage
}

func (r *HTTPRepositoryReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.HTTPRepository{}).
		WithOptions(options).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=httprepositories,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles HTTPRepository and removes caches.
func (r *HTTPRepositoryReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconciling")

	repo := &infrav1.HTTPRepository{}

	err := r.Client.Get(ctx, req.NamespacedName, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err, "Error retrieving HTTPRepository")
		return reconcile.Result{}, err
	}

	if !repo.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, repo)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(repo, infrav1.HTTPRepositoryFinalizer) {
		controllerutil.AddFinalizer(repo, infrav1.HTTPRepositoryFinalizer)
		return ctrl.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *HTTPRepositoryReconciler) reconcileDelete(_ context.Context, repo *infrav1.HTTPRepository) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(repo, infrav1.HTTPRepositoryFinalizer) {
		return ctrl.Result{}, nil
	}
	err := r.Storage.GetHTTPFileGetter(*repo).RemoveCache()
	if err != nil {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(repo, infrav1.HTTPRepositoryFinalizer)
	return ctrl.Result{}, nil
}
