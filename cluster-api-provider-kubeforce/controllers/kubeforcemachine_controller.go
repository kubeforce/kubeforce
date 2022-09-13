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

	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/kubeadm"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/ansible"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/assets"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/cloudinit"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/rand"
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
)

// KubeforceMachineReconciler reconciles a KubeforceMachine object
type KubeforceMachineReconciler struct {
	Client           client.Client
	Tracker          *remote.ClusterCacheTracker
	Log              logr.Logger
	AgentClientCache *agentctrl.ClientCache
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
	ctx = context.WithValue(ctx, "id", rand.String(3))
	r.Log.Info("reconciling",
		"id", ctx.Value("id"),
		"req", req,
	)

	log := r.Log.WithValues("req", req)
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

	if !controllerutil.ContainsFinalizer(kubeforceMachine, infrav1.MachineFinalizer) ||
		!controllerutil.ContainsFinalizer(kubeforceMachine, metav1.FinalizerDeleteDependents) {
		controllerutil.AddFinalizer(kubeforceMachine, infrav1.MachineFinalizer)
		controllerutil.AddFinalizer(kubeforceMachine, metav1.FinalizerDeleteDependents)
		return ctrl.Result{}, nil
	}

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

	// Check if the infrastructure is ready, otherwise return and wait for the cluster object to be updated
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for KubeforceCluster Controller to create cluster infrastructure")
		conditions.MarkFalse(
			kubeforceMachine,
			infrav1.PlaybooksCompletedCondition,
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
			infrav1.PlaybooksCompletedCondition,
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
			infrav1.PlaybooksCompletedCondition,
			bootstrapv1.DataSecretAvailableCondition,
		}},
	)
}

func (r *KubeforceMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, kubeforceMachine *infrav1.KubeforceMachine, kubeforceCluster *infrav1.KubeforceCluster) (res ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)
	config, err := kubeadm.GetConfig(ctx, r.Client, kubeforceMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	if config == nil {
		log.Info("Waiting for Controller to set OwnerRef on KubeforceMachine")
		return ctrl.Result{}, nil
	}
	if result, err := r.reconcileAgentRef(ctx, kubeforceMachine); !result.IsZero() || err != nil {
		if err != nil {
			log.Error(err, "failed to reconcile agent tls certificate")
		}
		return result, err
	}
	kfAgent := &infrav1.KubeforceAgent{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: kubeforceMachine.Namespace,
		Name:      kubeforceMachine.Spec.AgentRef.Name,
	}, kfAgent); err != nil {
		conditions.MarkFalse(kubeforceMachine, infrav1.AgentProvisionedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	agentClient, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		return ctrl.Result{}, err
	}

	if kubeforceMachine.Status.InternalIP == "" {
		info, err := agentClient.AgentV1alpha1().SysInfos().Get(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to set InternalIP")
		}
		kubeforceMachine.Status.InternalIP = info.Spec.Network.InternalIP
	}

	ready, err := r.reconcilePlaybooks(ctx, infrav1.PlaybooksCompletedCondition, kubeforceMachine, kfAgent, r.createPlaybookGenerators(config, kfAgent, kubeforceMachine, kubeforceCluster))
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
			conditions.MarkFalse(kubeforceMachine, bootstrapv1.DataSecretAvailableCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")
			return ctrl.Result{}, nil
		}

		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data for KubeforceMachine")
		conditions.MarkFalse(kubeforceMachine, bootstrapv1.DataSecretAvailableCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	} else {
		conditions.MarkTrue(kubeforceMachine, bootstrapv1.DataSecretAvailableCondition)
	}

	// Usually a cloud provider will do this, but there is no kubeforce-cloud provider.
	// Set ProviderID so the Cluster API Machine Controller can pull it
	if kubeforceMachine.Spec.ProviderID == nil {
		providerID := r.providerID(kubeforceMachine)
		if err := r.setNodeProviderID(ctx, cluster, agentClient, providerID); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to set providerID")
		}
		kubeforceMachine.Spec.ProviderID = &providerID
		kubeforceMachine.Status.Ready = true
	}

	return ctrl.Result{}, nil
}

// providerID return the provider identifier for this machine.
func (r *KubeforceMachineReconciler) providerID(m *infrav1.KubeforceMachine) string {
	return fmt.Sprintf("kf://%s", m.Name)
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
		conditions.MarkFalse(kfMachine, infrav1.CleanersCompletedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return false, err
	}
	// wait 20 seconds for the agent to be ready
	if !agent.IsReady(kfAgent) && time.Since(kfMachine.DeletionTimestamp.Time) < time.Second*20 {
		conditions.MarkFalse(kfMachine, infrav1.CleanersCompletedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, "Wait for agent ready")
		return false, nil
	}

	if agent.IsReady(kfAgent) {
		ready, err := r.reconcilePlaybooks(ctx, infrav1.CleanersCompletedCondition, kfMachine, kfAgent, r.cleanerGenerators())
		if err != nil {
			return false, err
		}
		if !ready {
			return false, nil
		}
	}

	delete(kfAgent.Labels, infrav1.AgentMachineLabel)
	// use optimistic concurrent update here
	if err := r.Client.Update(ctx, kfAgent); err != nil {
		conditions.MarkFalse(kfMachine, infrav1.CleanersCompletedCondition, infrav1.AgentProvisioningFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return false, err
	}
	return true, nil
}

func (r *KubeforceMachineReconciler) reconcilePlaybooks(ctx context.Context, t clusterv1.ConditionType, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, playbooks []*playbookGenerator) (bool, error) {
	// to avoid duplication, new playbooks should be added to the status immediately
	patchHelper, err := patch.NewHelper(kfMachine, r.Client)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := patchHelper.Patch(ctx, kfMachine); err != nil {
			r.Log.
				WithValues("machine", kfMachine.Name).
				Error(err, "failed to patch object")
		}
	}()
	for _, playbookGen := range playbooks {
		ready := false
		if playbookGen.Deployment {
			var err error
			ready, err = r.reconcilePlaybookDeployment(ctx, kfMachine, kfAgent, playbookGen)
			if err != nil {
				conditions.MarkFalse(kfMachine, t, infrav1.PlaybookDeployingFailedReason, clusterv1.ConditionSeverityError, err.Error())
				return false, err
			}
		} else {
			var err error
			ready, err = r.reconcilePlaybook(ctx, kfMachine, kfAgent, playbookGen)
			if err != nil {
				conditions.MarkFalse(kfMachine, t, infrav1.PlaybookDeployingFailedReason, clusterv1.ConditionSeverityError, err.Error())
				return false, err
			}
		}
		if !ready {
			msg := fmt.Sprintf("waiting for playbook with role: %s", playbookGen.Role)
			conditions.MarkFalse(kfMachine, t, infrav1.WaitingForCompletionPhaseReason, clusterv1.ConditionSeverityInfo, msg)
			return false, nil
		}
	}
	conditions.MarkTrue(kfMachine, t)
	return true, nil
}

func (r *KubeforceMachineReconciler) reconcilePlaybookDeployment(ctx context.Context, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, playbookGen *playbookGenerator) (bool, error) {
	role := playbookGen.Role
	pd, err := r.FindPlaybookDeploymentByRole(ctx, kfMachine, role)
	if err != nil {
		return false, err
	}
	if kfMachine.Status.Playbooks == nil {
		kfMachine.Status.Playbooks = make(map[string]*infrav1.PlaybookRefs)
	}
	playbookData, err := playbookGen.Generate(ctx)
	if err != nil {
		return false, err
	}
	if pd != nil {
		kfMachine.Status.Playbooks[role] = &infrav1.PlaybookRefs{
			Name:  pd.Name,
			Phase: pd.Status.ExternalPhase,
		}

		updated, err := r.updatePlaybookDeployment(ctx, pd, kfMachine, kfAgent, playbookData, role)
		if err != nil {
			return false, err
		}
		if updated {
			return false, nil
		}
		if conditions.IsTrue(pd, infrav1.SynchronizationCondition) && pd.Status.ExternalPhase == "Succeeded" {
			return true, nil
		}
		return false, nil
	}
	pd, err = r.createPlaybookDeployment(ctx, kfMachine, kfAgent, playbookData, role)
	if err != nil {
		return false, err
	}
	kfMachine.Status.Playbooks[role] = &infrav1.PlaybookRefs{
		Name:  pd.Name,
		Phase: pd.Status.ExternalPhase,
	}
	return false, nil
}

func (r *KubeforceMachineReconciler) reconcilePlaybook(ctx context.Context, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, playbookGen *playbookGenerator) (bool, error) {
	role := playbookGen.Role
	playbook, err := r.FindPlaybookByRole(ctx, kfMachine, role)
	if err != nil {
		return false, err
	}
	if kfMachine.Status.Playbooks == nil {
		kfMachine.Status.Playbooks = make(map[string]*infrav1.PlaybookRefs)
	}
	if playbook != nil {
		kfMachine.Status.Playbooks[role] = &infrav1.PlaybookRefs{
			Name:  playbook.Name,
			Phase: playbook.Status.ExternalPhase,
		}
		if conditions.IsTrue(playbook, infrav1.SynchronizationCondition) && playbook.Status.ExternalPhase == "Succeeded" {
			return true, nil
		}
		return false, nil
	}
	playbookData, err := playbookGen.Generate(ctx)
	if err != nil {
		return false, err
	}
	playbook, err = r.createPlaybook(ctx, kfMachine, kfAgent, playbookData, role)
	if err != nil {
		return false, err
	}
	kfMachine.Status.Playbooks[role] = &infrav1.PlaybookRefs{
		Name:  playbook.Name,
		Phase: playbook.Status.ExternalPhase,
	}
	return false, nil
}

func playbookLabelsByMachine(kfMachine *infrav1.KubeforceMachine, role string) map[string]string {
	return map[string]string{
		infrav1.PlaybookRoleLabelName:           role,
		infrav1.PlaybookAgentNameLabelName:      kfMachine.Spec.AgentRef.Name,
		infrav1.PlaybookControllerNameLabelName: kfMachine.Name,
		infrav1.PlaybookControllerKindLabelName: infrav1.GroupVersion.Group + ".KubeforceMachine",
	}
}

func (r *KubeforceMachineReconciler) FindPlaybookByRole(ctx context.Context, kfMachine *infrav1.KubeforceMachine, role string) (*infrav1.Playbook, error) {
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

func (r *KubeforceMachineReconciler) FindPlaybookDeploymentByRole(ctx context.Context, kfMachine *infrav1.KubeforceMachine, role string) (*infrav1.PlaybookDeployment, error) {
	list := &infrav1.PlaybookDeploymentList{}
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
		return nil, errors.Errorf("expected one PlaybookDeployment for role %s but found %d", role, len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *KubeforceMachineReconciler) updatePlaybookDeployment(ctx context.Context, pd *infrav1.PlaybookDeployment, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, data *ansible.Playbook, role string) (bool, error) {
	patchObj := client.MergeFrom(pd.DeepCopy())
	for key, value := range playbookLabelsByMachine(kfMachine, role) {
		pd.Labels[key] = value
	}
	pd.Spec.AgentRef = corev1.LocalObjectReference{
		Name: kfAgent.Name,
	}
	pd.Spec.Template.Spec = infrav1.RemotePlaybookSpec{
		Files:      data.Files,
		Entrypoint: data.Entrypoint,
	}

	changed, err := patchutil.HasChanges(patchObj, pd)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if changed {
		r.Log.Info("updating PlaybookDeployment",
			"id", ctx.Value("id"),
			"name", pd.Name)
		err := r.Client.Patch(ctx, pd, patchObj)
		if err != nil {
			return false, errors.Wrapf(err, "failed to patch PlaybookDeployment")
		}
		return true, nil
	}
	return false, nil
}

func (r *KubeforceMachineReconciler) createPlaybookDeployment(ctx context.Context, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, data *ansible.Playbook, role string) (*infrav1.PlaybookDeployment, error) {
	suffix := fmt.Sprintf("-%s-", role)
	pd := &infrav1.PlaybookDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(kfMachine.Name + suffix),
			Namespace: kfMachine.Namespace,
			Labels:    playbookLabelsByMachine(kfMachine, role),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1.GroupVersion.String(),
					Kind:               "KubeforceMachine",
					Name:               kfMachine.Name,
					UID:                kfMachine.UID,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
				},
			},
		},
		Spec: infrav1.PlaybookDeploymentSpec{
			AgentRef: corev1.LocalObjectReference{
				Name: kfAgent.Name,
			},
			Template: infrav1.PlaybookTemplateSpec{
				Spec: infrav1.RemotePlaybookSpec{
					Files:      data.Files,
					Entrypoint: data.Entrypoint,
				},
			},
			Paused: false,
		},
	}
	r.Log.Info("creating PlaybookDeployment",
		"id", ctx.Value("id"),
		"name", pd.Name)
	err := r.Client.Create(ctx, pd)
	if err != nil {
		return nil, err
	}
	return pd, nil
}

func (r *KubeforceMachineReconciler) createPlaybook(ctx context.Context, kfMachine *infrav1.KubeforceMachine, kfAgent *infrav1.KubeforceAgent, data *ansible.Playbook, role string) (*infrav1.Playbook, error) {
	suffix := fmt.Sprintf("-%s-", role)
	p := &infrav1.Playbook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(kfMachine.Name + suffix),
			Namespace: kfMachine.Namespace,
			Labels:    playbookLabelsByMachine(kfMachine, role),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1.GroupVersion.String(),
					Kind:               "KubeforceMachine",
					Name:               kfMachine.Name,
					UID:                kfMachine.UID,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
				},
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
	r.Log.Info("creating playbook",
		"id", ctx.Value("id"),
		"playbookName", p.Name)
	err := r.Client.Create(ctx, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type playbookGenerator struct {
	Role       string
	Deployment bool
	Generate   func(ctx context.Context) (*ansible.Playbook, error)
}

func (r *KubeforceMachineReconciler) cleanerGenerators() []*playbookGenerator {
	generators := make([]*playbookGenerator, 0)
	generators = append(generators, &playbookGenerator{
		Role: "cleanup",
		Generate: func(ctx context.Context) (*ansible.Playbook, error) {
			return assets.GetPlaybook(assets.PlaybookCleaner, nil)
		},
	})
	return generators
}

func (r *KubeforceMachineReconciler) getAPIServerEndpoints(kfMachine *infrav1.KubeforceMachine, kubeforceCluster *infrav1.KubeforceCluster) []string {
	_, isControlPlane := kfMachine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]
	if !isControlPlane {
		return kubeforceCluster.Status.APIServers
	}

	for _, apiServer := range kubeforceCluster.Status.APIServers {
		if apiServer == kfMachine.Status.InternalIP {
			return kubeforceCluster.Status.APIServers
		}
	}
	apiservers := kubeforceCluster.Status.APIServers
	apiservers = append(apiservers, kfMachine.Status.InternalIP)
	sort.Strings(apiservers)
	return apiservers
}

func (r *KubeforceMachineReconciler) createPlaybookGenerators(config kubeadm.Config, kfAgent *infrav1.KubeforceAgent,
	kfMachine *infrav1.KubeforceMachine, kubeforceCluster *infrav1.KubeforceCluster) []*playbookGenerator {
	generators := make([]*playbookGenerator, 0)
	generators = append(generators, &playbookGenerator{
		Role: "init",
		Generate: func(ctx context.Context) (*ansible.Playbook, error) {
			vars := make(map[string]interface{})
			vars["kubernetesVersion"] = config.GetKubernetesVersion()
			vars["targetArch"] = kfAgent.Spec.System.Arch
			return assets.GetPlaybook(assets.PlaybookInstaller, vars)
		},
	})
	generators = append(generators, &playbookGenerator{
		Role:       "loadbalancer",
		Deployment: true,
		Generate: func(ctx context.Context) (*ansible.Playbook, error) {
			vars := make(map[string]interface{})
			vars["apiServers"] = r.getAPIServerEndpoints(kfMachine, kubeforceCluster)
			vars["apiServerPort"] = "6443"
			vars["targetArch"] = kfAgent.Spec.System.Arch
			return assets.GetPlaybook(assets.PlaybookLoadbalancer, vars)
		},
	})
	// Make sure bootstrap data is available and populated.
	if config.IsDataAvailable() {
		generators = append(generators, &playbookGenerator{
			Role: "boot",
			Generate: func(ctx context.Context) (*ansible.Playbook, error) {
				data, err := config.GetBootstrapData(ctx)
				if err != nil {
					return nil, err
				}
				kubeadmConfig, err := config.GetKubeadmConfig(ctx)
				if err != nil {
					return nil, err
				}
				adapter := cloudinit.NewAnsibleAdapter(kubeadmConfig.Spec)
				return adapter.ToPlaybook(data)
			},
		})
	}
	return generators
}

// setNodeProviderID sets the kubeforce provider ID for the kubernetes node.
func (r *KubeforceMachineReconciler) setNodeProviderID(ctx context.Context,
	cluster *clusterv1.Cluster, agentClient *clientset.Clientset, providerID string) error {
	remoteClient, err := r.Tracker.GetClient(ctx, util.ObjectKey(cluster))
	if err != nil {
		return err
	}
	info, err := agentClient.AgentV1alpha1().SysInfos().Get(ctx)
	if err != nil {
		return err
	}
	if info.Spec.Network.Hostname == "" {
		return errors.New("hostname is empty")
	}
	key := client.ObjectKey{
		Name: info.Spec.Network.Hostname,
	}
	node := &corev1.Node{}
	if err := remoteClient.Get(ctx, key, node); err != nil {
		return err
	}
	if node.Spec.ProviderID != providerID {
		p := client.MergeFrom(node.DeepCopy())
		node.Spec.ProviderID = providerID

		if err := remoteClient.Patch(ctx, node, p); err != nil {
			return err
		}
	}
	return nil
}
