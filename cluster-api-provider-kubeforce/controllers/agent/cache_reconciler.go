package agent

import (
	"context"

	"github.com/pkg/errors"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// CacheReconciler is responsible for stopping remote agent caches when
// the agent is being deleted.
type CacheReconciler struct {
	Client      client.Client
	ClientCache *ClientCache
}

func (r *CacheReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceAgent{}).
		WithOptions(options).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{OwnerType: &infrav1.KubeforceAgent{}},
		).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

// Reconcile reconciles KubeforceAgent and removes client caches
func (r *CacheReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconciling")

	kfAgent := &infrav1.KubeforceAgent{}

	err := r.Client.Get(ctx, req.NamespacedName, kfAgent)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.ClientCache.DeleteHolder(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		log.Error(err, "Error retrieving KubeforceAgent")
		return reconcile.Result{}, err
	}
	if kfAgent.Spec.Addresses == nil || !kfAgent.Spec.Installed {
		return reconcile.Result{}, nil
	}
	oldChecksum := r.ClientCache.getChecksum(req.NamespacedName)
	if oldChecksum == "" {
		return reconcile.Result{}, nil
	}

	keys, err := agent.GetKeys(ctx, r.Client, kfAgent)
	if err != nil {
		return reconcile.Result{}, err
	}
	host, err := agent.GetServer(*kfAgent.Spec.Addresses)
	if err != nil {
		return reconcile.Result{}, err
	}
	calcChecksum, err := r.ClientCache.calcChecksum(host, keys)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "unable to calc checksum of restConfig for agent %q", req.NamespacedName)
	}
	if calcChecksum != oldChecksum {
		r.ClientCache.DeleteHolder(req.NamespacedName)
	}
	return reconcile.Result{}, nil
}
