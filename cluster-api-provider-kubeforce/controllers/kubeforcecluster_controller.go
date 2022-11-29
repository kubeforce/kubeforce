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
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
)

// KubeforceClusterReconciler reconciles a KubeforceCluster object.
type KubeforceClusterReconciler struct {
	Client client.Client
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;update;patch;watch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *KubeforceClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the KubeforceCluster instance
	kubeforceCluster := &infrav1.KubeforceCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubeforceCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kubeforceCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on KubeforceCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubeforceCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the KubeforceCluster object and status after each reconciliation.
	defer func() {
		if err := patchKubeforceCluster(ctx, patchHelper, kubeforceCluster); err != nil {
			log.Error(err, "failed to patch KubeforceCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(kubeforceCluster, infrav1.ClusterFinalizer) {
		controllerutil.AddFinalizer(kubeforceCluster, infrav1.ClusterFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !kubeforceCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, kubeforceCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, cluster, kubeforceCluster)
}

func patchKubeforceCluster(ctx context.Context, patchHelper *patch.Helper, kubeforceCluster *infrav1.KubeforceCluster) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process (instead we are hiding it during the deletion process).
	conditions.SetSummary(kubeforceCluster,
		conditions.WithConditions(
			infrav1.InfrastructureAvailableCondition,
		),
		conditions.WithStepCounterIf(kubeforceCluster.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		kubeforceCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.InfrastructureAvailableCondition,
		}},
	)
}

func (r *KubeforceClusterReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, kubeforceCluster *infrav1.KubeforceCluster) (ctrl.Result, error) {
	kubeforceCluster.Status.Ready = true
	conditions.MarkTrue(kubeforceCluster, infrav1.InfrastructureAvailableCondition)
	machines, err := r.getControlPlaneMachineList(ctx, kubeforceCluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.refreshAPIServers(ctx, kubeforceCluster, machines)
	if err := r.reconcileControlPlaneEndpoint(ctx, cluster, kubeforceCluster, machines); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeforceClusterReconciler) getExternalAddress(ctx context.Context, machines []infrav1.KubeforceMachine) ([]string, error) {
	adresses := make([]string, 0, len(machines))
	for _, kfMachine := range machines {
		if kfMachine.Spec.AgentRef != nil {
			kfAgent := &infrav1.KubeforceAgent{}
			if err := r.Client.Get(ctx, client.ObjectKey{
				Namespace: kfMachine.Namespace,
				Name:      kfMachine.Spec.AgentRef.Name,
			}, kfAgent); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return nil, errors.Wrapf(err, "unable to get agent for machine %s", kfMachine.Name)
			}
			if agent.IsHealthy(kfAgent) {
				address := stringutil.Find(stringutil.IsNotEmpty, kfAgent.Spec.Addresses.ExternalDNS, kfAgent.Spec.Addresses.ExternalIP)
				adresses = append(adresses, address)
			}
		}
	}
	return adresses, nil
}

func (r *KubeforceClusterReconciler) reconcileControlPlaneEndpoint(ctx context.Context, cluster *clusterv1.Cluster,
	kubeforceCluster *infrav1.KubeforceCluster, controlPlaneMachines []infrav1.KubeforceMachine) error {
	patchHelper, err := patch.NewHelper(kubeforceCluster, r.Client)
	if err != nil {
		return errors.WithStack(err)
	}
	updatedEndpoint, err := r.refreshControlPlaneEndpoint(ctx, kubeforceCluster, controlPlaneMachines)
	if err != nil {
		return errors.WithStack(err)
	}
	if !cluster.Spec.ControlPlaneEndpoint.IsValid() {
		return nil
	}
	if cluster.Spec.ControlPlaneEndpoint.Host == kubeforceCluster.Spec.ControlPlaneEndpoint.Host &&
		cluster.Spec.ControlPlaneEndpoint.Port == kubeforceCluster.Spec.ControlPlaneEndpoint.Port {
		return nil
	}
	if updatedEndpoint {
		if err := patchHelper.Patch(ctx, kubeforceCluster); err != nil {
			return errors.WithStack(err)
		}
	}

	// delete ControlPlaneEndpoint in cluster
	clusterPatchData := client.MergeFrom(cluster.DeepCopy())
	cluster.Spec.ControlPlaneEndpoint.Port = 0
	cluster.Spec.ControlPlaneEndpoint.Host = ""
	if err := r.Client.Patch(ctx, cluster, clusterPatchData); err != nil {
		return errors.WithStack(err)
	}

	if err := r.deleteKubeconfig(ctx, cluster); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
func (r *KubeforceClusterReconciler) deleteKubeconfig(ctx context.Context, cluster *clusterv1.Cluster) error {
	clusterName := util.ObjectKey(cluster)
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WithStack(err)
	}
	if err == nil {
		if err := r.Client.Delete(ctx, configSecret); err != nil {
			return errors.WithStack(err)
		}
		ctrl.LoggerFrom(ctx).Info("kubeconfig has been deleted")
	}
	return nil
}

func (r *KubeforceClusterReconciler) refreshControlPlaneEndpoint(ctx context.Context, cluster *infrav1.KubeforceCluster,
	controlPlaneMachines []infrav1.KubeforceMachine) (bool, error) {
	externalAddresses, err := r.getExternalAddress(ctx, controlPlaneMachines)
	if err != nil {
		return false, err
	}
	for _, address := range externalAddresses {
		if cluster.Spec.ControlPlaneEndpoint.Host == address {
			return false, nil
		}
	}
	if len(externalAddresses) == 0 {
		if !cluster.Spec.ControlPlaneEndpoint.IsValid() {
			cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
				Host: "0.0.0.0",
				Port: 9443,
			}
			return true, nil
		}

		return false, nil
	}
	sort.Strings(externalAddresses)
	cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: externalAddresses[0],
		Port: 9443,
	}
	return true, nil
}

func (r *KubeforceClusterReconciler) refreshAPIServers(_ context.Context, cluster *infrav1.KubeforceCluster, controlPlaneMachines []infrav1.KubeforceMachine) {
	apiServers := make([]string, 0, len(controlPlaneMachines))
	for _, m := range controlPlaneMachines {
		if m.Status.InternalIP != "" {
			apiServers = append(apiServers, m.Status.InternalIP)
		}
	}
	sort.Strings(apiServers)
	cluster.Status.APIServers = apiServers
}

func (r *KubeforceClusterReconciler) reconcileDelete(ctx context.Context, kubeforceCluster *infrav1.KubeforceCluster) (ctrl.Result, error) {
	// Set the LoadBalancerAvailableCondition reporting delete is started, and issue a patch in order to make
	// this visible to the users.
	patchHelper, err := patch.NewHelper(kubeforceCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(kubeforceCluster, infrav1.InfrastructureAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchKubeforceCluster(ctx, patchHelper, kubeforceCluster); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch KubeforceCluster")
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(kubeforceCluster, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *KubeforceClusterReconciler) getControlPlaneMachineList(ctx context.Context, cluster *infrav1.KubeforceCluster) ([]infrav1.KubeforceMachine, error) {
	ml := &infrav1.KubeforceMachineList{}
	if err := r.Client.List(
		ctx,
		ml,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.MachineControlPlaneLabelName: "",
			clusterv1.ClusterLabelName:             cluster.Name,
		},
	); err != nil {
		return nil, errors.Wrap(err, "failed to list machines")
	}

	machines := make([]infrav1.KubeforceMachine, 0, len(ml.Items))
	for _, machine := range ml.Items {
		if machine.DeletionTimestamp.IsZero() {
			machines = append(machines, machine)
		}
	}

	return machines, nil
}

// KubeforceMachineToKubeforceCluster is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for KubeforceCluster based on updates to a KubeforceMachine.
func (r *KubeforceClusterReconciler) KubeforceMachineToKubeforceCluster(o client.Object) []ctrl.Request {
	m, ok := o.(*infrav1.KubeforceMachine)
	if !ok {
		r.Log.Info(fmt.Sprintf("Expected a KubeforceMachine but got a %T", o))
		return nil
	}
	_, isControlPlane := m.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]
	infraClusterName := m.Labels[infrav1.KubeforceClusterLabelName]
	if isControlPlane && infraClusterName != "" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: m.Namespace, Name: infraClusterName}}}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeforceClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := ctrl.Log
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceCluster{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Build(r)

	if err != nil {
		return err
	}
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("KubeforceCluster"), mgr.GetClient(), &infrav1.KubeforceCluster{})),
		predicates.ClusterUnpaused(logger),
	)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &infrav1.KubeforceMachine{}},
		handler.EnqueueRequestsFromMapFunc(r.KubeforceMachineToKubeforceCluster),
		predicates.ResourceNotPaused(logger),
	)
}
