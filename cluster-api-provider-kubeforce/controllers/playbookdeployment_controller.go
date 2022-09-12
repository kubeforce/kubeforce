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

	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"

	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiagent "k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	agentclient "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
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

// PlaybookDeploymentReconciler reconciles a PlaybookDeployment object
type PlaybookDeploymentReconciler struct {
	Log              logr.Logger
	Client           client.Client
	AgentClientCache *agentctrl.ClientCache
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbookdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbookdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=playbookdeployments/finalizers,verbs=update

// Reconcile handles PlaybookDeployment events.
func (r *PlaybookDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := r.Log.WithValues("pd", req)
	pd := &infrav1.PlaybookDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, pd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(pd, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the PlaybookDeployment object and status after each reconciliation.
	defer func() {
		if err := patchPlaybookDeployment(ctx, patchHelper, pd); err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			log.Error(err, "failed to patch PlaybookDeployment")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !pd.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, pd)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(pd, infrav1.PlaybookDeploymentFinalizer) {
		controllerutil.AddFinalizer(pd, infrav1.PlaybookDeploymentFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle non-deleted playbooks
	return r.reconcileNormal(ctx, pd)
}

func patchPlaybookDeployment(ctx context.Context, patchHelper *patch.Helper, playbook *infrav1.PlaybookDeployment) error {
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
func (r *PlaybookDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.PlaybookDeployment{}).
		Watches(
			&source.Kind{Type: &infrav1.KubeforceAgent{}},
			handler.EnqueueRequestsFromMapFunc(r.KubeforceAgentToPlaybookDeployments),
		).
		Complete(r)
}

func (r *PlaybookDeploymentReconciler) KubeforceAgentToPlaybookDeployments(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	a, ok := o.(*infrav1.KubeforceAgent)
	if !ok {
		r.Log.Info(fmt.Sprintf("Expected a KubeforceAgent but got a %T", o))
		return nil
	}

	pdLabels := map[string]string{infrav1.PlaybookAgentNameLabelName: a.Name}
	pdList := &infrav1.PlaybookDeploymentList{}
	if err := r.Client.List(context.TODO(), pdList, client.InNamespace(a.Namespace), client.MatchingLabels(pdLabels)); err != nil {
		return nil
	}
	for _, m := range pdList.Items {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

func (r *PlaybookDeploymentReconciler) GetKubeforceAgent(ctx context.Context, playbook *infrav1.PlaybookDeployment) (*infrav1.KubeforceAgent, error) {
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

func (r *PlaybookDeploymentReconciler) reconcileDelete(ctx context.Context, playbook *infrav1.PlaybookDeployment) (ctrl.Result, error) {
	if err := r.deleteExternalPlaybookDeployment(ctx, playbook); err != nil {
		msg := fmt.Sprintf("unable to delete external PlaybookDeployment. err: %v", err.Error())
		conditions.MarkFalse(playbook, infrav1.SynchronizationCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(playbook, infrav1.PlaybookDeploymentFinalizer)
	return ctrl.Result{}, nil
}

func (r *PlaybookDeploymentReconciler) deleteExternalPlaybookDeployment(ctx context.Context, playbook *infrav1.PlaybookDeployment) error {
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
	err = clientSet.AgentV1alpha1().PlaybookDeployments().Delete(ctx, playbook.Status.ExternalName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *PlaybookDeploymentReconciler) reconcileNormal(ctx context.Context, pd *infrav1.PlaybookDeployment) (ctrl.Result, error) {
	log := r.Log.WithValues("pd", capiutil.ObjectKey(pd))
	// Fetch the Machine.
	kfAgent, err := r.GetKubeforceAgent(ctx, pd)
	if err != nil {
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	pd.Labels[infrav1.PlaybookAgentNameLabelName] = kfAgent.Name
	if r.shouldAdopt(pd) {
		pd.OwnerReferences = capiutil.EnsureOwnerRef(pd.OwnerReferences,
			*metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent")),
		)
	}
	// Return early if the object or agent is paused.
	if annotations.HasPaused(kfAgent) || annotations.HasPaused(pd) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	if !agent.IsReady(kfAgent) {
		msg := "agent is not ready"
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.WaitingForAgentReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{}, nil
	}
	agentClient, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	extPlaybookDeployment, err := r.findExternalPlaybookDeployment(ctx, agentClient, pd)
	if err != nil {
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if extPlaybookDeployment == nil {
		if pd.Status.ExternalName != "" {
			msg := fmt.Sprintf("extarnal PlaybookDeployment: %q is not found", pd.Status.ExternalName)
			conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
			return ctrl.Result{}, nil
		}
		externalPlaybookDeployment, err := r.createExternalPlaybookDeployment(ctx, agentClient, pd)
		if err != nil {
			conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		pd.Status.ExternalName = externalPlaybookDeployment.Name
		msg := "external PlaybookDeployment has been created"
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.WaitingForObservedGenerationReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	if pd.Status.ExternalName != "" && pd.Status.ExternalName != extPlaybookDeployment.Name {
		msg := fmt.Sprintf("extarnal pd: %s is not equal to specified %s", extPlaybookDeployment.Name, pd.Status.ExternalName)
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{}, nil
	}
	pd.Status.ExternalName = extPlaybookDeployment.Name
	pd.Status.ExternalPhase = string(extPlaybookDeployment.Status.Phase)
	updated, err := r.updateExternalPlaybookDeployment(ctx, agentClient, extPlaybookDeployment, pd)
	if err != nil {
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.SynchronizationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if updated {
		msg := "external PlaybookDeployment has been updated"
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.WaitingForObservedGenerationReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	if extPlaybookDeployment.Generation != extPlaybookDeployment.Status.ObservedGeneration {
		msg := fmt.Sprintf("observedGeneration %d is not equal generation %d yet", extPlaybookDeployment.Status.ObservedGeneration, extPlaybookDeployment.Generation)
		conditions.MarkFalse(pd, infrav1.SynchronizationCondition, infrav1.WaitingForObservedGenerationReason, clusterv1.ConditionSeverityInfo, msg)
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	conditions.MarkTrue(pd, infrav1.SynchronizationCondition)
	if extPlaybookDeployment.Status.Phase == v1alpha1.PlaybookDeploymentProgressing {
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	return ctrl.Result{}, nil
}

func externalPlaybookDeploymentLabels(playbook *infrav1.PlaybookDeployment) map[string]string {
	return map[string]string{
		apiagent.PlaybookControllerNameLabelName: playbook.Name,
		apiagent.PlaybookControllerKindLabelName: infrav1.GroupVersion.Group + ".PlaybookDeployment",
	}
}

func (r *PlaybookDeploymentReconciler) findExternalPlaybookDeployment(ctx context.Context, agentClient *agentclient.Clientset, playbook *infrav1.PlaybookDeployment) (*v1alpha1.PlaybookDeployment, error) {
	list, err := agentClient.AgentV1alpha1().PlaybookDeployments().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(externalPlaybookDeploymentLabels(playbook)).String(),
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
		return nil, errors.Errorf("expected one playbookDeployment, but found %d", len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *PlaybookDeploymentReconciler) createExternalPlaybookDeployment(ctx context.Context, agentClient *agentclient.Clientset, pd *infrav1.PlaybookDeployment) (*v1alpha1.PlaybookDeployment, error) {
	agentPlaybookDeployment := &v1alpha1.PlaybookDeployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pd.Name + "-",
			Namespace:    pd.Namespace,
			Labels:       externalPlaybookDeploymentLabels(pd),
			Annotations:  map[string]string{},
		},
		Spec: toExternalPlaybookDeploymentSpec(pd.Spec),
	}
	resultPlaybookDeployment, err := agentClient.AgentV1alpha1().PlaybookDeployments().Create(ctx, agentPlaybookDeployment, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return resultPlaybookDeployment, err
}

func toExternalPlaybookDeploymentSpec(pdSpec infrav1.PlaybookDeploymentSpec) v1alpha1.PlaybookDeploymentSpec {
	return v1alpha1.PlaybookDeploymentSpec{
		Template: v1alpha1.PlaybookTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      pdSpec.Template.Labels,
				Annotations: pdSpec.Template.Annotations,
			},
			Spec: v1alpha1.PlaybookSpec{
				Files:      pdSpec.Template.Spec.Files,
				Entrypoint: pdSpec.Template.Spec.Entrypoint,
			},
		},
		RevisionHistoryLimit: pdSpec.RevisionHistoryLimit,
		Paused:               pdSpec.Paused,
	}
}

func (r *PlaybookDeploymentReconciler) updateExternalPlaybookDeployment(
	ctx context.Context, agentClient *agentclient.Clientset,
	extPd *v1alpha1.PlaybookDeployment, pd *infrav1.PlaybookDeployment) (bool, error) {
	patchObj := client.MergeFrom(extPd.DeepCopy())
	extPd.Spec.Template.ObjectMeta.Labels = pd.Spec.Template.Labels
	extPd.Spec.Template.ObjectMeta.Annotations = pd.Spec.Template.Annotations
	extPd.Spec.Template.Spec.Files = pd.Spec.Template.Spec.Files
	extPd.Spec.Template.Spec.Entrypoint = pd.Spec.Template.Spec.Entrypoint
	extPd.Spec.Paused = pd.Spec.Paused
	if pd.Spec.RevisionHistoryLimit != nil {
		extPd.Spec.RevisionHistoryLimit = pd.Spec.RevisionHistoryLimit
	}

	changed, err := patchutil.HasChanges(patchObj, extPd)
	if err != nil {
		return false, errors.WithStack(err)
	}

	diff, err := patchObj.Data(extPd)
	if err != nil {
		return false, errors.Wrapf(err, "failed to calculate patch data")
	}

	if changed {
		_, err := agentClient.AgentV1alpha1().PlaybookDeployments().Patch(ctx, extPd.Name, patchObj.Type(), diff, metav1.PatchOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "failed to patch PlaybookDeployment")
		}
		return true, nil
	}
	return false, nil
}

func (r *PlaybookDeploymentReconciler) shouldAdopt(p *infrav1.PlaybookDeployment) bool {
	return metav1.GetControllerOf(p) == nil && !capiutil.HasOwner(p.OwnerReferences, infrav1.GroupVersion.String(), []string{"KubeforceAgent"})
}
