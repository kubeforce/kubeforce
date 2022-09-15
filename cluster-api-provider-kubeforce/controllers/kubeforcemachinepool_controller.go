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

// Package controllers implements controller functionality.
package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	utilexp "sigs.k8s.io/cluster-api/exp/util"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/names"
)

// KubeforceMachinePoolReconciler reconciles a KubeforceMachinePool object.
type KubeforceMachinePoolReconciler struct {
	Client client.Client
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachinepools/status;kubeforcemachinepools/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *KubeforceMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, rerr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the KubeforceMachinePool instance.
	kubeforceMachinePool := &infrav1.KubeforceMachinePool{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubeforceMachinePool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the MachinePool.
	machinePool, err := utilexp.GetOwnerMachinePool(ctx, r.Client, kubeforceMachinePool.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machinePool == nil {
		log.Info("Waiting for MachinePool Controller to set OwnerRef on KubeforceMachinePool")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine-pool", machinePool.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubeforceMachinePool, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the KubeforceMachinePool object and status after each reconciliation.
	defer func() {
		if err := patchKubeforceMachinePool(ctx, patchHelper, kubeforceMachinePool); err != nil {
			log.Error(err, "failed to patch KubeforceMachinePool")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !kubeforceMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePool, kubeforceMachinePool)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(kubeforceMachinePool, infrav1.MachinePoolFinalizer) {
		controllerutil.AddFinalizer(kubeforceMachinePool, infrav1.MachinePoolFinalizer)
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Info("KubeforceMachinePool owner MachinePool is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}

	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine pool with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, cluster, machinePool, kubeforceMachinePool)
}

// SetupWithManager will add watches for this controller.
func (r *KubeforceMachinePoolReconciler) SetupWithManager(logger logr.Logger, mgr ctrl.Manager, options controller.Options) error {
	clusterToKubeforceMachinePools, err := util.ClusterToObjectsMapper(mgr.GetClient(), &infrav1.KubeforceMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return err
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceMachinePool{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Watches(
			&source.Kind{Type: &expv1.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(utilexp.MachinePoolToInfrastructureMapFunc(
				infrav1.GroupVersion.WithKind("KubeforceMachinePool"), logger)),
		).
		Watches(
			&source.Kind{Type: &infrav1.KubeforceMachine{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &infrav1.KubeforceMachinePool{},
				IsController: true,
			},
		).
		Build(r)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToKubeforceMachinePools),
		predicates.ClusterUnpausedAndInfrastructureReady(logger),
	)
}

func (r *KubeforceMachinePoolReconciler) reconcileDelete(_ context.Context, _ *expv1.MachinePool, kubeforceMachinePool *infrav1.KubeforceMachinePool) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(kubeforceMachinePool, infrav1.MachinePoolFinalizer)
	return ctrl.Result{}, nil
}

func (r *KubeforceMachinePoolReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, machinePool *expv1.MachinePool, kubeforceMachinePool *infrav1.KubeforceMachinePool) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	// Make sure bootstrap data is available and populated.
	if machinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data for MachinePool")
		return ctrl.Result{}, nil
	}
	machines, err := r.machinesByMachinePool(ctx, cluster.Name, kubeforceMachinePool)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get machines by the machine pool")
	}
	desiredReplicas := int(*machinePool.Spec.Replicas)
	desiredMachines := r.desiredMachines(cluster.Name, kubeforceMachinePool, desiredReplicas)
	desiredMachineMap := machinesToMap(desiredMachines)
	// delete machines
	for _, machine := range machines {
		if _, ok := desiredMachineMap[machine.Name]; !ok {
			if err := r.Client.Delete(ctx, machine); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	// add machines
	machineMap := machinesToMap(machines)
	for _, desiredMachine := range desiredMachines {
		_, ok := machineMap[desiredMachine.Name]
		if !ok {
			if err := r.Client.Create(ctx, desiredMachine); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "unable to create a KubeforceMachine")
			}
		}
	}
	machines, err = r.machinesByMachinePool(ctx, cluster.Name, kubeforceMachinePool)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get machines by the machine pool")
	}

	readyReplicas := 0
	waitForMachines := make([]string, 0)
	kubeforceMachinePool.Spec.ProviderIDList = []string{}
	for _, machine := range machines {
		if machine.Spec.ProviderID != nil && machine.Status.Ready && machine.Status.ObservedGeneration == machine.Generation {
			readyReplicas++
			kubeforceMachinePool.Spec.ProviderIDList = append(kubeforceMachinePool.Spec.ProviderIDList, *machine.Spec.ProviderID)
		} else {
			waitForMachines = append(waitForMachines, machine.Name)
		}
	}

	kubeforceMachinePool.Status.Replicas = int32(desiredReplicas)

	if kubeforceMachinePool.Spec.ProviderID == "" {
		kubeforceMachinePool.Spec.ProviderID = getKubeforceMachinePoolProviderID(cluster.Name, kubeforceMachinePool.Name)
	}

	kubeforceMachinePool.Status.Ready = readyReplicas == desiredReplicas
	if kubeforceMachinePool.Status.Ready {
		conditions.MarkTrue(kubeforceMachinePool, clusterv1.InfrastructureReadyCondition)
	} else {
		conditions.MarkFalse(
			kubeforceMachinePool,
			clusterv1.InfrastructureReadyCondition,
			clusterv1.WaitingForInfrastructureFallbackReason,
			clusterv1.ConditionSeverityInfo,
			"waiting for KubeforceMachines with names %q", waitForMachines)
	}
	return ctrl.Result{}, nil
}

func buildKubeforceMachineLabels(sourceLabels map[string]string, clusterName string) map[string]string {
	labels := make(map[string]string)
	for key, val := range sourceLabels {
		labels[key] = val
	}
	labels[clusterv1.ClusterLabelName] = clusterName
	return labels
}

func (r *KubeforceMachinePoolReconciler) machinesByMachinePool(ctx context.Context, clusterName string, kubeforceMachinePool *infrav1.KubeforceMachinePool) ([]*infrav1.KubeforceMachine, error) {
	list := &infrav1.KubeforceMachineList{}
	listOpts := []client.ListOption{
		client.InNamespace(kubeforceMachinePool.Namespace),
		client.MatchingLabels(map[string]string{
			clusterv1.ClusterLabelName: clusterName,
		}),
	}
	err := r.Client.List(ctx, list, listOpts...)
	if err != nil {
		return nil, err
	}
	machines := make([]*infrav1.KubeforceMachine, 0)
	for _, machine := range list.Items {
		if metav1.IsControlledBy(&machine, kubeforceMachinePool) {
			machines = append(machines, machine.DeepCopy())
		}
	}
	return machines, nil
}

func (r *KubeforceMachinePoolReconciler) desiredMachines(clusterName string, kubeforceMachinePool *infrav1.KubeforceMachinePool, replicas int) []*infrav1.KubeforceMachine {
	machines := make([]*infrav1.KubeforceMachine, 0, replicas)
	labels := buildKubeforceMachineLabels(kubeforceMachinePool.Spec.Template.ObjectMeta.Labels, clusterName)
	for i := 0; i < replicas; i++ {
		suffix := fmt.Sprintf("-%d", i)
		spec := kubeforceMachinePool.Spec.Template.Spec.DeepCopy()
		m := &infrav1.KubeforceMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        names.BuildName(kubeforceMachinePool.Name, suffix),
				Namespace:   kubeforceMachinePool.Namespace,
				Labels:      labels,
				Annotations: kubeforceMachinePool.Spec.Template.ObjectMeta.Annotations,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "KubeforceMachinePool",
						Name:       kubeforceMachinePool.Name,
						UID:        kubeforceMachinePool.UID,
						Controller: pointer.BoolPtr(true),
					},
				},
			},
			Spec: *spec,
		}
		machines = append(machines, m)
	}
	return machines
}

func getKubeforceMachinePoolProviderID(clusterName, kubeforceMachinePoolName string) string {
	return fmt.Sprintf("kf:////%s-kmp-%s", clusterName, kubeforceMachinePoolName)
}

func patchKubeforceMachinePool(ctx context.Context, patchHelper *patch.Helper, kubeforceMachinePool *infrav1.KubeforceMachinePool) error {
	conditions.SetSummary(kubeforceMachinePool,
		conditions.WithConditions(
			clusterv1.InfrastructureReadyCondition,
		),
		conditions.WithStepCounterIf(kubeforceMachinePool.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		kubeforceMachinePool,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			clusterv1.InfrastructureReadyCondition,
		}},
	)
}

func machinesToMap(machines []*infrav1.KubeforceMachine) map[string]*infrav1.KubeforceMachine {
	m := make(map[string]*infrav1.KubeforceMachine, len(machines))
	for _, machine := range machines {
		m[machine.Name] = machine
	}
	return m
}
