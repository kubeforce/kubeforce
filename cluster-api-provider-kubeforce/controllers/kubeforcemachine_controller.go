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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/kubeadm"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/playbook"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/ansible"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/cloudinit"
)

// KubeforceMachineReconciler reconciles a KubeforceMachine object.
type KubeforceMachineReconciler struct {
	TemplateReconciler playbook.TemplateReconciler
	Client             client.Client
	Tracker            *remote.ClusterCacheTracker
	Log                logr.Logger
	AgentClientCache   *agentctrl.ClientCache
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles KubeforceMachine events.
func (r *KubeforceMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	kubeforceMachine := &infrav1.KubeforceMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubeforceMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	r.Log.Info("reconciling KubeforceMachine", "req", req)
	log := r.Log.WithValues("req", req)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, kubeforceMachine.ObjectMeta)
	if err != nil {
		log.Info("KubeforceMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, kubeforceMachine) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}
	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubeforceMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the DockerMachine object and status after each reconciliation.
	defer func() {
		if err := patchKubeforceMachine(ctx, patchHelper, kubeforceMachine); err != nil {
			log.Error(err, "failed to patch KubeforceMachine")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !kubeforceMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, kubeforceMachine)
	}

	if !controllerutil.ContainsFinalizer(kubeforceMachine, infrav1.MachineFinalizer) {
		controllerutil.AddFinalizer(kubeforceMachine, infrav1.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	// Check if the infrastructure is ready, otherwise return and wait for the cluster object to be updated
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for KubeforceCluster Controller to create cluster infrastructure")
		conditions.MarkFalse(
			kubeforceMachine,
			infrav1.InitPlaybooksCondition,
			infrav1.WaitingForClusterInfrastructureReason,
			clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Fetch the Kubeforce Cluster.
	kubeforceCluster := &infrav1.KubeforceCluster{}
	kubeforceClusterName := client.ObjectKey{
		Namespace: kubeforceMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, kubeforceClusterName, kubeforceCluster); err != nil {
		log.Info("KubeforceCluster is not available yet")
		return ctrl.Result{}, nil
	}
	kubeforceMachine.Labels[infrav1.KubeforceClusterLabelName] = kubeforceCluster.Name

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, cluster, kubeforceMachine, kubeforceCluster)
}

func patchKubeforceMachine(ctx context.Context, patchHelper *patch.Helper, kubeforceMachine *infrav1.KubeforceMachine) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process (instead we are hiding the step counter during the deletion process).
	conditions.SetSummary(kubeforceMachine,
		conditions.WithConditions(
			infrav1.AgentProvisionedCondition,
			infrav1.InitPlaybooksCondition,
			infrav1.BootstrapExecSucceededCondition,
			infrav1.ProviderIDSucceededCondition,
			bootstrapv1.DataSecretAvailableCondition,
		),
		conditions.WithStepCounterIf(kubeforceMachine.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		kubeforceMachine,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.AgentProvisionedCondition,
			infrav1.InitPlaybooksCondition,
			infrav1.BootstrapExecSucceededCondition,
			infrav1.ProviderIDSucceededCondition,
			bootstrapv1.DataSecretAvailableCondition,
		}},
	)
}

func (r *KubeforceMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, kfm *infrav1.KubeforceMachine, kfc *infrav1.KubeforceCluster) (res ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)
	config, err := kubeadm.GetConfig(ctx, r.Client, kfm)
	if err != nil {
		return ctrl.Result{}, err
	}
	if config == nil {
		log.Info("Waiting for Controller to set OwnerRef on KubeforceMachine")
		return ctrl.Result{}, nil
	}
	if result, err := r.reconcileAgentRef(ctx, kfm); !result.IsZero() || err != nil {
		if err != nil {
			log.Error(err, "failed to reconcile agent tls certificate")
		}
		return result, err
	}
	kfAgent := &infrav1.KubeforceAgent{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: kfm.Namespace,
		Name:      kfm.Spec.AgentRef.Name,
	}, kfAgent); err != nil {
		conditions.MarkFalse(kfm, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if kfAgent.Status.SystemInfo != nil {
		kfm.Status.DefaultIPAddress = kfAgent.Status.SystemInfo.Network.DefaultIPAddress
	}

	vars := make(map[string]interface{})
	// variables for the loadbalancer are not needed if it is disabled
	if kfc.Spec.Loadbalancer == nil || !kfc.Spec.Loadbalancer.Disabled {
		vars["apiServers"] = r.getAPIServerEndpoints(kfm, kfc)
		vars["apiServerPort"] = "6443"
	}
	vars["kubernetesVersion"] = config.GetKubernetesVersion()
	vars["targetArch"] = kfAgent.Spec.System.Arch
	vars["systemInfo"] = kfAgent.Status.SystemInfo

	ready, err := r.TemplateReconciler.Reconcile(ctx, kfm, infrav1.TemplateTypeInstall, vars)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if !config.IsDataAvailable() {
		if !config.IsControlPlane() && !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
			log.Info("Waiting for the control plane to be initialized")
			conditions.MarkFalse(kfm, bootstrapv1.DataSecretAvailableCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")
			return ctrl.Result{}, nil
		}

		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data for KubeforceMachine")
		conditions.MarkFalse(kfm, bootstrapv1.DataSecretAvailableCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}
	conditions.MarkTrue(kfm, bootstrapv1.DataSecretAvailableCondition)

	ready, err = r.reconcileCloudInitPlaybook(ctx, config, kfm, kfAgent, vars)
	if err != nil {
		conditions.MarkFalse(kfm, infrav1.BootstrapExecSucceededCondition, infrav1.BootstrappingReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{}, nil
	}
	// Usually a cloud provider will do this, but there is no kubeforce-cloud provider.
	// Set ProviderID so the Cluster API Machine Controller can pull it
	err = r.reconcileNodeProviderID(ctx, kfm, cluster, kfAgent)
	if err != nil {
		conditions.MarkFalse(kfm, infrav1.ProviderIDSucceededCondition, infrav1.ProviderIDFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(kfm, infrav1.ProviderIDSucceededCondition)
	kfm.Status.Ready = true

	return ctrl.Result{}, nil
}

// providerID return the provider identifier for this machine.
func (r *KubeforceMachineReconciler) providerID(m *infrav1.KubeforceMachine) string {
	return fmt.Sprintf("kf://%s", m.Spec.AgentRef.Name)
}

func (r *KubeforceMachineReconciler) reconcileAgentRef(ctx context.Context, kfMachine *infrav1.KubeforceMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	if kfMachine.Spec.AgentRef == nil && kfMachine.Spec.AgentSelector == nil {
		log.Info("Waiting for the agentRef or agentSelector to be initialized")
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if kfMachine.Spec.AgentRef != nil {
		kfAgent := &infrav1.KubeforceAgent{}
		if err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: kfMachine.Namespace,
			Name:      kfMachine.Spec.AgentRef.Name,
		}, kfAgent); err != nil {
			conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		if kfAgent.Labels[infrav1.AgentMachineLabel] != kfMachine.Name {
			msg := fmt.Sprintf("%q label has the wrong value found: %q expected: %q", infrav1.AgentMachineLabel, kfAgent.Labels[infrav1.AgentMachineLabel], kfMachine.Name)
			conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, msg)
			return ctrl.Result{}, errors.New(msg)
		}
		if !agent.IsReady(kfAgent) {
			conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, "agent is not ready")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		conditions.MarkTrue(kfMachine, infrav1.AgentProvisionedCondition)
		return ctrl.Result{}, nil
	}

	// find an agent that was blocked by this machine but not stored in AgentRef
	list := &infrav1.KubeforceAgentList{}
	listOpts := []client.ListOption{
		client.InNamespace(kfMachine.Namespace),
		client.MatchingLabels(map[string]string{
			infrav1.AgentMachineLabel: kfMachine.Name,
		}),
	}
	err := r.Client.List(ctx, list, listOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(list.Items) > 1 {
		msg := fmt.Sprintf("more than one agent found by label %s=%s", infrav1.AgentMachineLabel, kfMachine.Name)
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, errors.New(msg)
	}
	if len(list.Items) == 1 {
		kfMachine.Spec.AgentRef = &corev1.LocalObjectReference{
			Name: list.Items[0].Name,
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// find free agent
	agentSelector, err := metav1.LabelSelectorAsSelector(kfMachine.Spec.AgentSelector)
	if err != nil {
		log.Error(err, "failed to get agent label selector")
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	agents, err := agentsBySelector(ctx, r.Client, kfMachine.Namespace, agentSelector)
	if err != nil {
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	kfAgent := findAgent(agents, func(a *infrav1.KubeforceAgent) bool {
		return a.Labels[infrav1.AgentMachineLabel] == "" && agent.IsReady(a)
	})
	if kfAgent == nil {
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, "no free agent")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	kfAgent.Labels[infrav1.AgentMachineLabel] = kfMachine.Name
	kfAgent.Labels[clusterv1.ClusterLabelName] = kfMachine.Labels[clusterv1.ClusterLabelName]
	// use optimistic concurrent update here
	if err := r.Client.Update(ctx, kfAgent); err != nil {
		conditions.MarkFalse(kfMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	kfMachine.Spec.AgentRef = &corev1.LocalObjectReference{
		Name: kfAgent.Name,
	}
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (r *KubeforceMachineReconciler) reconcileDelete(ctx context.Context, kubeforceMachine *infrav1.KubeforceMachine) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(kubeforceMachine, infrav1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}
	// Set the AgentProvisionedCondition reporting delete is started, and issue a patch in order to make
	// this visible to the users.
	patchHelper, err := patch.NewHelper(kubeforceMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(kubeforceMachine, infrav1.AgentProvisionedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchKubeforceMachine(ctx, patchHelper, kubeforceMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch KubeforceMachine")
	}
	ready, err := r.reconcileCleaner(ctx, kubeforceMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{}, nil
	}

	// Machine is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(kubeforceMachine, infrav1.MachineFinalizer)
	return ctrl.Result{}, nil
}

// KubeforceClusterToKubeforceMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of KubeforceMachines.
func (r *KubeforceMachineReconciler) KubeforceClusterToKubeforceMachines(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*infrav1.KubeforceCluster)
	if !ok {
		r.Log.Info(fmt.Sprintf("Expected a KubeforceCluster but got a %T", o))
		return nil
	}

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		return result
	}

	machineLabels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), machineList, client.InNamespace(c.Namespace), client.MatchingLabels(machineLabels)); err != nil {
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

type agentFilter func(*infrav1.KubeforceAgent) bool

func findAgent(agents []*infrav1.KubeforceAgent, filterFn agentFilter) *infrav1.KubeforceAgent {
	for _, kfAgent := range agents {
		if filterFn(kfAgent) {
			return kfAgent
		}
	}
	return nil
}

// SetupWithManager sets up watches for this controller.
func (r *KubeforceMachineReconciler) SetupWithManager(logger logr.Logger, mgr ctrl.Manager, options controller.Options) error {
	clusterToKubeforceMachines, err := util.ClusterToObjectsMapper(mgr.GetClient(), &infrav1.KubeforceMachineList{}, mgr.GetScheme())
	if err != nil {
		return err
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceMachine{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("KubeforceMachine"))),
		).
		Watches(
			&source.Kind{Type: &infrav1.KubeforceCluster{}},
			handler.EnqueueRequestsFromMapFunc(r.KubeforceClusterToKubeforceMachines),
		).
		Watches(
			&source.Kind{Type: &infrav1.Playbook{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &infrav1.KubeforceMachine{},
				IsController: true,
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.PlaybookDeployment{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &infrav1.KubeforceMachine{},
				IsController: true,
			},
		).
		Build(r)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToKubeforceMachines),
		predicates.ClusterUnpausedAndInfrastructureReady(logger),
	)
}

func (r *KubeforceMachineReconciler) reconcileCleaner(ctx context.Context, kfMachine *infrav1.KubeforceMachine) (bool, error) {
	if kfMachine.Spec.AgentRef == nil {
		return true, nil
	}
	kfAgent := &infrav1.KubeforceAgent{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: kfMachine.Namespace,
		Name:      kfMachine.Spec.AgentRef.Name,
	}, kfAgent); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		conditions.MarkFalse(kfMachine, infrav1.CleanupPlaybooksCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return false, err
	}
	// wait 20 seconds for the agent to be ready
	if !agent.IsHealthy(kfAgent) && time.Since(kfMachine.DeletionTimestamp.Time) < time.Second*20 {
		conditions.MarkFalse(kfMachine, infrav1.CleanupPlaybooksCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, "Wait for agent ready")
		return false, nil
	}

	if agent.IsHealthy(kfAgent) {
		ready, err := r.TemplateReconciler.Reconcile(ctx, kfMachine, infrav1.TemplateTypeDelete, nil)
		if err != nil {
			return false, err
		}
		if !ready {
			return false, nil
		}
	}

	delete(kfAgent.Labels, infrav1.AgentMachineLabel)
	delete(kfAgent.Labels, clusterv1.ClusterLabelName)
	// use optimistic concurrent update here
	if err := r.Client.Update(ctx, kfAgent); err != nil {
		conditions.MarkFalse(kfMachine, infrav1.CleanupPlaybooksCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return false, err
	}
	return true, nil
}

func (r *KubeforceMachineReconciler) reconcileCloudInitPlaybook(ctx context.Context, config kubeadm.Config,
	kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, vars map[string]interface{}) (bool, error) {
	pb, err := r.findPlaybookByRole(ctx, kfMachine, "boot")
	if err != nil {
		return false, err
	}
	if pb != nil {
		if conditions.IsTrue(pb, infrav1.SynchronizationCondition) && pb.Status.ExternalPhase == "Succeeded" {
			// Update the BootstrapExecSucceededCondition condition
			conditions.MarkTrue(kfMachine, infrav1.BootstrapExecSucceededCondition)
			return true, nil
		}
		conditions.MarkFalse(kfMachine, infrav1.BootstrapExecSucceededCondition, infrav1.BootstrappingReason, clusterv1.ConditionSeverityWarning, pb.Status.ExternalPhase)
		return false, nil
	}
	data, err := config.GetBootstrapData(ctx)
	if err != nil {
		return false, err
	}
	kubeadmConfig, err := config.GetKubeadmConfig(ctx)
	if err != nil {
		return false, err
	}
	adapter := cloudinit.NewAnsibleAdapter(kubeadmConfig.Spec)
	playbookData, err := adapter.ToPlaybook(data)
	if err != nil {
		return false, err
	}

	pb, err = r.createPlaybook(ctx, kfMachine, kfAgent, playbookData, "boot", vars)
	if err != nil {
		return false, err
	}
	conditions.MarkFalse(kfMachine, infrav1.BootstrapExecSucceededCondition, infrav1.BootstrappingReason, clusterv1.ConditionSeverityWarning, pb.Status.ExternalPhase)
	return false, nil
}

func playbookLabelsByMachine(kfMachine *infrav1.KubeforceMachine, role string) map[string]string {
	return map[string]string{
		clusterv1.ClusterLabelName:              kfMachine.Labels[clusterv1.ClusterLabelName],
		infrav1.PlaybookRoleLabelName:           role,
		infrav1.PlaybookAgentNameLabelName:      kfMachine.Spec.AgentRef.Name,
		infrav1.PlaybookControllerNameLabelName: kfMachine.Name,
		infrav1.PlaybookControllerKindLabelName: infrav1.GroupVersion.Group + ".KubeforceMachine",
	}
}

func (r *KubeforceMachineReconciler) findPlaybookByRole(ctx context.Context, kfMachine *infrav1.KubeforceMachine, role string) (*infrav1.Playbook, error) {
	list := &infrav1.PlaybookList{}
	listOptions := client.MatchingLabelsSelector{
		Selector: labels.Set(playbookLabelsByMachine(kfMachine, role)).AsSelector(),
	}
	err := r.Client.List(ctx, list, listOptions)
	if err != nil && apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, errors.Errorf("expected one Playbook for role %s but found %d", role, len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *KubeforceMachineReconciler) createPlaybook(ctx context.Context, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, data *ansible.Playbook, role string, vars map[string]interface{}) (*infrav1.Playbook, error) {
	suffix := fmt.Sprintf("-%s-", role)
	p := &infrav1.Playbook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(kfMachine.Name + suffix),
			Namespace: kfMachine.Namespace,
			Labels:    playbookLabelsByMachine(kfMachine, role),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(kfMachine, infrav1.GroupVersion.WithKind("KubeforceMachine")),
			},
		},
		Spec: infrav1.PlaybookSpec{
			AgentRef: corev1.LocalObjectReference{
				Name: kfAgent.Name,
			},
			RemotePlaybookSpec: infrav1.RemotePlaybookSpec{
				Files:      data.Files,
				Entrypoint: data.Entrypoint,
			},
		},
	}
	if len(vars) > 0 {
		varsData, err := yaml.Marshal(vars)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to marshal variables for Playbook %s", p.Name)
		}
		p.Spec.Files["variables.yaml"] = string(varsData)
	}
	r.Log.Info("creating playbook", "key", client.ObjectKeyFromObject(p))
	err := r.Client.Create(ctx, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *KubeforceMachineReconciler) getAPIServerEndpoints(kfMachine *infrav1.KubeforceMachine, kubeforceCluster *infrav1.KubeforceCluster) []string {
	_, isControlPlane := kfMachine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]
	apiServers := kubeforceCluster.Status.APIServers
	if apiServers == nil {
		apiServers = make([]string, 0)
	}
	if !isControlPlane {
		return apiServers
	}

	for _, apiServer := range apiServers {
		if apiServer == kfMachine.Status.DefaultIPAddress {
			return apiServers
		}
	}

	apiServers = append(apiServers, kfMachine.Status.DefaultIPAddress)
	sort.Strings(apiServers)
	return apiServers
}

func (r *KubeforceMachineReconciler) reconcileNodeProviderID(ctx context.Context, kfMachine *infrav1.KubeforceMachine,
	cluster *clusterv1.Cluster, kfAgent *infrav1.KubeforceAgent) error {
	if kfMachine.Spec.ProviderID != nil {
		return nil
	}
	if kfAgent.Status.SystemInfo == nil || kfAgent.Status.SystemInfo.Network.Hostname == "" {
		return errors.New("hostname is empty")
	}
	remoteClient, err := r.Tracker.GetClient(ctx, util.ObjectKey(cluster))
	if err != nil {
		return err
	}
	node := &corev1.Node{}
	if err := remoteClient.Get(ctx, client.ObjectKey{Name: kfAgent.Status.SystemInfo.Network.Hostname}, node); err != nil {
		return err
	}

	if node.Spec.ProviderID != "" {
		kfMachine.Spec.ProviderID = pointer.String(node.Spec.ProviderID)
		return nil
	}
	providerID := r.providerID(kfMachine)
	p := client.MergeFrom(node.DeepCopy())
	node.Spec.ProviderID = providerID
	if err := remoteClient.Patch(ctx, node, p); err != nil {
		return err
	}
	kfMachine.Spec.ProviderID = pointer.String(node.Spec.ProviderID)
	return nil
}
