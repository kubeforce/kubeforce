/*
Copyright 2021.

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

	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiagent "k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	agentclient "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	agentconditions "k3f.io/kubeforce/agent/pkg/util/conditions"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// PlaybookReconciler reconciles a Playbook object
type PlaybookReconciler struct {
	Log              logr.Logger
	Client           client.Client
	AgentClientCache *agentctrl.ClientCache
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbooks/finalizers,verbs=update

// Reconcile handles Playbook events.
func (r *PlaybookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := r.Log.WithValues("playbook", req)
	playbook := &infrav1.Playbook{}
	if err := r.Client.Get(ctx, req.NamespacedName, playbook); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(playbook, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the Playbook object and status after each reconciliation.
	defer func() {
		if err := patchPlaybook(ctx, patchHelper, playbook); err != nil {
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
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process
	// (instead we are hiding the step counter during the deletion process).
	conditions.SetSummary(playbook,
		conditions.WithConditions(
			infrav1.SynchronizationCondition,
		),
		conditions.WithStepCounterIf(playbook.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		playbook,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.SynchronizationCondition,
		}},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaybookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Playbook{}).
		Complete(r)
}

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
	if err := r.deleteExternalPlaybook(ctx, playbook); err != nil {
		msg := fmt.Sprintf("unable delete external playbook. err: %v", err.Error())
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(playbook, infrav1.PlaybookFinalizer)
	return ctrl.Result{}, nil
}

func (r *PlaybookReconciler) deleteExternalPlaybook(ctx context.Context, playbook *infrav1.Playbook) error {
	if playbook.Status.ExternalName == "" {
		return nil
	}
	kfAgent, err := r.GetKubeforceAgent(ctx, playbook)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if kfAgent == nil || !agent.IsReady(kfAgent) {
		return nil
	}
	clientSet, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		return err
	}
	err = clientSet.AgentV1alpha1().Playbooks().Delete(ctx, playbook.Status.ExternalName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *PlaybookReconciler) reconcileNormal(ctx context.Context, playbook *infrav1.Playbook) (ctrl.Result, error) {
	log := r.Log.WithValues("playbook", capiutil.ObjectKey(playbook))
	// Fetch the Machine.
	kfAgent, err := r.GetKubeforceAgent(ctx, playbook)
	if err != nil {
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if r.shouldAdopt(playbook) {
		playbook.OwnerReferences = capiutil.EnsureOwnerRef(
			playbook.OwnerReferences,
			*metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent")),
		)
	}
	// Return early if the object or agent is paused.
	if annotations.HasPausedAnnotation(kfAgent) || annotations.HasPausedAnnotation(playbook) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	if !agent.IsReady(kfAgent) {
		msg := "agent is not ready"
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	agentClient, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	extPlaybook, err := r.findExternalPlaybook(ctx, agentClient, playbook)
	if err != nil {
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if extPlaybook == nil {
		if playbook.Status.ExternalName != "" {
			msg := fmt.Sprintf("extarnal playbook: %s is not found", playbook.Status.ExternalName)
			conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
			return ctrl.Result{}, nil
		}
		externalPlaybook, err := r.createExternalPlaybook(ctx, agentClient, playbook)
		if err != nil {
			conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		playbook.Status.ExternalName = externalPlaybook.Name
		msg := "external PlaybookDeployment has been created"
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.WaitingForObservedGenerationReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	if playbook.Status.ExternalName != "" && playbook.Status.ExternalName != extPlaybook.Name {
		msg := fmt.Sprintf("extarnal playbook: %s is not equal to specified %s", extPlaybook.Name, playbook.Status.ExternalName)
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, nil
	}
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
	return resultPlaybook, err
}

func (r *PlaybookReconciler) shouldAdopt(p *infrav1.Playbook) bool {
	return metav1.GetControllerOf(p) == nil && !capiutil.HasOwner(p.OwnerReferences, infrav1.GroupVersion.String(), []string{"KubeforceAgent"})
}
