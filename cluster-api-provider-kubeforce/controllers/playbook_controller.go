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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiagent "k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	agentclient "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	agentconditions "k3f.io/kubeforce/agent/pkg/util/conditions"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
)

// PlaybookReconciler reconciles a Playbook object.
type PlaybookReconciler struct {
	Log              logr.Logger
	Client           client.Client
	AgentClientCache *agentctrl.ClientCache
}

const (
	waitForAgentMsg = "Waiting for the agent to be ready"
)

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks/finalizers,verbs=update

// Reconcile handles Playbook events.
func (r *PlaybookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	if ctx.Err() != nil {
		return reconcile.Result{}, nil
	}
	log := r.Log.WithValues("playbook", req)
	playbook := &infrav1.Playbook{}
	if err := r.Client.Get(ctx, req.NamespacedName, playbook); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.Client, playbook.ObjectMeta)
	if err != nil && errors.Cause(err) != capiutil.ErrNoCluster {
		log.Error(err, "unable to get cluster for Playbook", "playbook", req)
		return ctrl.Result{}, err
	}

	if cluster != nil {
		log = log.WithValues("cluster", cluster.Name)
	}

	// Return early if the object or Cluster is paused.
	if cluster != nil && cluster.Spec.Paused || annotations.HasPaused(playbook) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(playbook, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the Playbook object and status after each reconciliation.
	defer func() {
		r.reconcilePhase(playbook)
		// We want to save the last status even if the context was closed.
		if err := patchPlaybook(context.Background(), patchHelper, playbook); err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			log.Error(err, "failed to patch Playbook")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !playbook.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, playbook)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(playbook, infrav1.PlaybookFinalizer) {
		controllerutil.AddFinalizer(playbook, infrav1.PlaybookFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle non-deleted playbooks
	return r.reconcileNormal(ctx, playbook)
}

func patchPlaybook(ctx context.Context, patchHelper *patch.Helper, playbook *infrav1.Playbook) error {
	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		playbook,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			infrav1.SynchronizationCondition,
		}},
	)
}

func (r *PlaybookReconciler) reconcilePhase(pb *infrav1.Playbook) {
	// Set the phase to "failed" if any of Status.FailureReason or Status.FailureMessage is not-nil.
	if pb.Status.FailureReason != "" || pb.Status.FailureMessage != "" {
		pb.Status.Phase = infrav1.PlaybookPhaseFailed
		return
	}

	// Set the phase to "deleting" if the deletion timestamp is set.
	if !pb.DeletionTimestamp.IsZero() {
		pb.Status.Phase = infrav1.PlaybookPhaseDeleting
		return
	}

	extFailedCond := conditions.Get(pb, clusterv1.ConditionType(v1alpha1.PlaybookFailedCondition))
	if extFailedCond != nil && extFailedCond.Status == corev1.ConditionTrue {
		pb.Status.Phase = infrav1.PlaybookPhaseFailed
		pb.Status.FailureReason = infrav1.PlaybookStatusError(extFailedCond.Reason)
		pb.Status.FailureMessage = extFailedCond.Message
		return
	}

	if conditions.IsTrue(pb, clusterv1.ConditionType(v1alpha1.PlaybookExecutionCondition)) {
		pb.Status.Phase = infrav1.PlaybookPhaseCompleted
		return
	}

	if conditions.IsTrue(pb, infrav1.SynchronizationCondition) {
		pb.Status.Phase = infrav1.PlaybookPhaseSynchronization
		return
	}

	if pb.Status.Phase == "" {
		pb.Status.Phase = infrav1.PlaybookPhaseProvisioning
		return
	}

	pb.Status.Phase = infrav1.PlaybookPhaseUnknown
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaybookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Playbook{}).
		Watches(
			&source.Kind{Type: &infrav1.KubeforceAgent{}},
			handler.EnqueueRequestsFromMapFunc(r.KubeforceAgentToPlaybook),
		).
		Build(r)
	if err != nil {
		return err
	}
	clusterToPlaybooks, err := capiutil.ClusterToObjectsMapper(mgr.GetClient(), &infrav1.PlaybookList{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToPlaybooks),
		predicates.ClusterUnpaused(r.Log),
	)
	if err != nil {
		return errors.Wrap(err, "failed to add Watch for Clusters to controller manager")
	}
	return nil
}

// KubeforceAgentToPlaybook is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for Playbook to update when one of the KubeforceAgent gets updated.
func (r *PlaybookReconciler) KubeforceAgentToPlaybook(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	a, ok := o.(*infrav1.KubeforceAgent)
	if !ok {
		r.Log.Info(fmt.Sprintf("Expected a KubeforceAgent but got a %T", o))
		return nil
	}

	pdLabels := map[string]string{infrav1.PlaybookAgentNameLabelName: a.Name}
	pdList := &infrav1.PlaybookList{}
	if err := r.Client.List(context.TODO(), pdList, client.InNamespace(a.Namespace), client.MatchingLabels(pdLabels)); err != nil {
		return nil
	}
	for _, m := range pdList.Items {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

// GetKubeforceAgent returns the KubeforceAgent that controls this Playbook.
func (r *PlaybookReconciler) GetKubeforceAgent(ctx context.Context, playbook *infrav1.Playbook) (*infrav1.KubeforceAgent, error) {
	objectKey := client.ObjectKey{
		Namespace: playbook.Namespace,
		Name:      playbook.Spec.AgentRef.Name,
	}
	kfAgent := &infrav1.KubeforceAgent{}
	err := r.Client.Get(ctx, objectKey, kfAgent)
	if err != nil {
		return nil, err
	}
	return kfAgent, nil
}

func (r *PlaybookReconciler) reconcileDelete(ctx context.Context, playbook *infrav1.Playbook) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(playbook, infrav1.PlaybookFinalizer) {
		return ctrl.Result{}, nil
	}
	if playbook.Status.ExternalName == "" {
		controllerutil.RemoveFinalizer(playbook, infrav1.PlaybookFinalizer)
		return ctrl.Result{}, nil
	}
	kfAgent, err := r.GetKubeforceAgent(ctx, playbook)
	if err != nil {
		playbook.Status.FailureMessage = err.Error()
		playbook.Status.FailureReason = infrav1.DeletePlaybookError
		return ctrl.Result{}, err
	}
	// wait 60 seconds for the agent to be ready
	if !agent.IsHealthy(kfAgent) && time.Since(playbook.DeletionTimestamp.Time) < time.Second*60 {
		msg := waitForAgentMsg
		playbook.Status.FailureMessage = msg
		playbook.Status.FailureReason = infrav1.AgentIsNotReadyPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, nil
	}

	// wait for forced deletion
	if !agent.IsHealthy(kfAgent) && kfAgent.DeletionTimestamp.IsZero() {
		msg := fmt.Sprintf("Waiting for the agent to be ready. If you want to force deletion then remove the %q KubeforceAgent", client.ObjectKeyFromObject(kfAgent))
		playbook.Status.FailureMessage = msg
		playbook.Status.FailureReason = infrav1.AgentIsNotReadyPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, nil
	}

	if !agent.IsHealthy(kfAgent) {
		controllerutil.RemoveFinalizer(playbook, infrav1.PlaybookFinalizer)
		return ctrl.Result{}, nil
	}
	result, err := r.reconcileDeleteExternalPlaybook(ctx, playbook, kfAgent)
	if err != nil {
		msg := fmt.Sprintf("unable to delete external playbook. err: %v", err.Error())
		playbook.Status.FailureMessage = msg
		playbook.Status.FailureReason = infrav1.DeletePlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, err
	}
	playbook.Status.FailureMessage = ""
	playbook.Status.FailureReason = ""
	if !result.IsZero() {
		return result, nil
	}
	controllerutil.RemoveFinalizer(playbook, infrav1.PlaybookFinalizer)
	return ctrl.Result{}, nil
}

func (r *PlaybookReconciler) reconcileDeleteExternalPlaybook(ctx context.Context, p *infrav1.Playbook, a *infrav1.KubeforceAgent) (ctrl.Result, error) {
	clientSet, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(a))
	if err != nil {
		return ctrl.Result{}, err
	}
	extPlaybook, err := clientSet.AgentV1alpha1().Playbooks().Get(ctx, p.Status.ExternalName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !extPlaybook.DeletionTimestamp.IsZero() {
		conditions.MarkTrue(p, infrav1.SynchronizationCondition)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	err = clientSet.AgentV1alpha1().Playbooks().Delete(ctx, p.Status.ExternalName, metav1.DeleteOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkTrue(p, infrav1.SynchronizationCondition)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *PlaybookReconciler) reconcileNormal(ctx context.Context, playbook *infrav1.Playbook) (ctrl.Result, error) {
	log := r.Log.WithValues("playbook", capiutil.ObjectKey(playbook))

	// we don't need to sync if the external playbook has reached the termination phase
	if conditions.IsTrue(playbook, clusterv1.ConditionType(v1alpha1.PlaybookExecutionCondition)) ||
		conditions.IsTrue(playbook, clusterv1.ConditionType(v1alpha1.PlaybookFailedCondition)) {
		return ctrl.Result{}, nil
	}
	// Fetch the Agent.
	kfAgent, err := r.GetKubeforceAgent(ctx, playbook)
	if err != nil {
		playbook.Status.FailureMessage = fmt.Sprintf("unable to get KubeforceAgent err: %v", err)
		playbook.Status.FailureReason = infrav1.AgentClientPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	playbook.Labels[infrav1.PlaybookAgentNameLabelName] = kfAgent.Name
	if r.shouldAdopt(playbook) {
		playbook.OwnerReferences = capiutil.EnsureOwnerRef(
			playbook.OwnerReferences,
			*metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent")),
		)
	}
	// Return early if the agent is paused.
	if annotations.HasPaused(kfAgent) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	if !agent.IsHealthy(kfAgent) {
		playbook.Status.FailureMessage = waitForAgentMsg
		playbook.Status.FailureReason = infrav1.AgentIsNotReadyPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, waitForAgentMsg)
		return ctrl.Result{}, nil
	}

	agentClient, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		playbook.Status.FailureMessage = fmt.Sprintf("unable to get agent ClientSet err: %v", err)
		playbook.Status.FailureReason = infrav1.AgentClientPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	extPlaybook, err := r.findExternalPlaybook(ctx, agentClient, playbook)
	if err != nil {
		playbook.Status.FailureMessage = fmt.Sprintf("unable to find external Playbook err: %v", err)
		playbook.Status.FailureReason = infrav1.ExternalPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if extPlaybook == nil {
		if playbook.Status.ExternalName != "" {
			msg := fmt.Sprintf("extarnal playbook: %s is not found", playbook.Status.ExternalName)
			playbook.Status.FailureMessage = msg
			playbook.Status.FailureReason = infrav1.ExternalPlaybookError
			conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
			return ctrl.Result{}, nil
		}
		externalPlaybook, err := r.createExternalPlaybook(ctx, agentClient, playbook)
		if err != nil {
			playbook.Status.FailureMessage = fmt.Sprintf("unable to create ExternalPlaybook err: %v", err)
			playbook.Status.FailureReason = infrav1.ExternalPlaybookError
			conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		playbook.Status.ExternalName = externalPlaybook.Name
		playbook.Status.ExternalPhase = string(externalPlaybook.Status.Phase)
		playbook.Status.FailureMessage = ""
		playbook.Status.FailureReason = ""
		conditions.MarkTrue(playbook, infrav1.SynchronizationCondition)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	if playbook.Status.ExternalName != "" && playbook.Status.ExternalName != extPlaybook.Name {
		msg := fmt.Sprintf("extarnal playbook: %s is not equal to specified %s", extPlaybook.Name, playbook.Status.ExternalName)
		playbook.Status.FailureMessage = msg
		playbook.Status.FailureReason = infrav1.ExternalPlaybookError
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, nil
	}
	playbook.Status.FailureMessage = ""
	playbook.Status.FailureReason = ""
	playbook.Status.ExternalName = extPlaybook.Name
	playbook.Status.ExternalPhase = string(extPlaybook.Status.Phase)
	appendExternalConditions(playbook, extPlaybook)
	conditions.MarkTrue(playbook, infrav1.SynchronizationCondition)
	if agentconditions.IsTrue(extPlaybook, v1alpha1.PlaybookExecutionCondition) || agentconditions.IsTrue(extPlaybook, v1alpha1.PlaybookFailedCondition) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{
		RequeueAfter: 10 * time.Second,
	}, nil
}

func appendExternalConditions(pl *infrav1.Playbook, extPl *v1alpha1.Playbook) {
	for _, condition := range extPl.Status.Conditions {
		conditions.Set(pl, &clusterv1.Condition{
			Type:    clusterv1.ConditionType(condition.Type),
			Status:  condition.Status,
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
}

func externalPlaybookLabels(playbook *infrav1.Playbook) map[string]string {
	return map[string]string{
		apiagent.PlaybookControllerNameLabelName: playbook.Name,
		apiagent.PlaybookControllerKindLabelName: infrav1.GroupVersion.Group + ".Playbook",
	}
}

func (r *PlaybookReconciler) findExternalPlaybook(ctx context.Context, agentClient *agentclient.Clientset, playbook *infrav1.Playbook) (*v1alpha1.Playbook, error) {
	list, err := agentClient.AgentV1alpha1().Playbooks().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(externalPlaybookLabels(playbook)).String(),
	})
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
		return nil, errors.Errorf("expected one playbook, but found %d", len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *PlaybookReconciler) createExternalPlaybook(ctx context.Context, agentClient *agentclient.Clientset, playbook *infrav1.Playbook) (*v1alpha1.Playbook, error) {
	agentPlaybook := &v1alpha1.Playbook{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: playbook.Name + "-",
			Namespace:    playbook.Namespace,
			Labels:       externalPlaybookLabels(playbook),
			Annotations:  map[string]string{},
		},
		Spec: v1alpha1.PlaybookSpec{
			Files:      playbook.Spec.Files,
			Entrypoint: playbook.Spec.Entrypoint,
		},
	}
	resultPlaybook, err := agentClient.AgentV1alpha1().Playbooks().Create(ctx, agentPlaybook, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return resultPlaybook, nil
}

func (r *PlaybookReconciler) shouldAdopt(p *infrav1.Playbook) bool {
	return metav1.GetControllerOf(p) == nil && !capiutil.HasOwner(p.OwnerReferences, infrav1.GroupVersion.String(), []string{"KubeforceAgent"})
}
