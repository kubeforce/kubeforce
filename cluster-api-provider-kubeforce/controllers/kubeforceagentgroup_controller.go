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
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
)

// KubeforceAgentGroupReconciler reconciles a KubeforceAgentGroup object.
type KubeforceAgentGroupReconciler struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceagentgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceagentgroups/status;kubeforceagentgroups/finalizers,verbs=get;update;patch

// Reconcile reconciles KubeforceAgentGroup.
func (r *KubeforceAgentGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, rerr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the KubeforceAgentGroup instance.
	agentGroup := &infrav1.KubeforceAgentGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, agentGroup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log = log.WithValues("agent-group", agentGroup.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(agentGroup, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the KubeforceAgentGroup object and status after each reconciliation.
	defer func() {
		if err := patchKubeforceAgentGroup(ctx, patchHelper, agentGroup); err != nil {
			log.Error(err, "failed to patch KubeforceAgentGroup")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	if !agentGroup.ObjectMeta.DeletionTimestamp.IsZero() {
		r.reconcileDelete(ctx, agentGroup)
		return ctrl.Result{}, nil
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(agentGroup, infrav1.AgentGroupFinalizer) ||
		!controllerutil.ContainsFinalizer(agentGroup, metav1.FinalizerDeleteDependents) {
		controllerutil.AddFinalizer(agentGroup, infrav1.AgentGroupFinalizer)
		controllerutil.AddFinalizer(agentGroup, metav1.FinalizerDeleteDependents)
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, agentGroup)
}

// SetupWithManager will add watches for this controller.
func (r *KubeforceAgentGroupReconciler) SetupWithManager(logger logr.Logger, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceAgentGroup{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Watches(
			&source.Kind{Type: &infrav1.KubeforceAgent{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &infrav1.KubeforceAgentGroup{},
				IsController: true,
			},
		).
		Complete(r)
}

func (r *KubeforceAgentGroupReconciler) reconcileDelete(_ context.Context, agentGroup *infrav1.KubeforceAgentGroup) {
	if controllerutil.ContainsFinalizer(agentGroup, infrav1.AgentGroupFinalizer) {
		controllerutil.RemoveFinalizer(agentGroup, infrav1.AgentGroupFinalizer)
	}
}

func (r *KubeforceAgentGroupReconciler) reconcileNormal(ctx context.Context, agentGroup *infrav1.KubeforceAgentGroup) (ctrl.Result, error) {
	desiredAgents := r.desiredAgents(agentGroup)
	agents, err := agentsInGroup(ctx, r.Client, agentGroup)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get agents by the group")
	}

	desiredAgentMap := agentsToMap(desiredAgents)
	// delete agents
	for _, agent := range agents {
		if _, ok := desiredAgentMap[agent.Name]; !ok {
			if err := r.Client.Delete(ctx, agent); err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
		}
	}
	// add or update agents
	agentMap := agentsToMap(agents)
	for _, desiredAgent := range desiredAgents {
		agent, ok := agentMap[desiredAgent.Name]
		if ok {
			if err := r.updateAgent(ctx, agent, desiredAgent); err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
		} else {
			if err := r.Client.Create(ctx, desiredAgent); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "unable to create a KubeforceAgent")
			}
		}
	}
	// get the agents to update status
	agents, err = agentsInGroup(ctx, r.Client, agentGroup)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to get agents by the group")
	}

	readyReplicas := 0
	waitForAgents := make([]string, 0)
	for _, agent := range agents {
		if agent.Status.Phase == infrav1.AgentPhaseRunning &&
			conditions.IsTrue(agent, infrav1.HealthyCondition) &&
			agent.Status.ObservedGeneration == agent.Generation {
			readyReplicas++
		} else {
			waitForAgents = append(waitForAgents, agent.Name)
		}
	}

	agentGroup.Status.Replicas = int32(len(agentGroup.Spec.Addresses))
	agentGroup.Status.ReadyReplicas = int32(readyReplicas)

	agentGroup.Status.Ready = agentGroup.Status.Replicas == agentGroup.Status.ReadyReplicas
	if agentGroup.Status.Ready {
		conditions.MarkTrue(agentGroup, clusterv1.InfrastructureReadyCondition)
	} else {
		conditions.MarkFalse(
			agentGroup,
			clusterv1.InfrastructureReadyCondition,
			clusterv1.WaitingForInfrastructureFallbackReason,
			clusterv1.ConditionSeverityInfo,
			"waiting for KubeforceAgents with names %q", waitForAgents)
	}
	return ctrl.Result{}, nil
}

func buildKubeforceAgentLabels(sourceLabels map[string]string, agentGroup string) map[string]string {
	agentLabels := make(map[string]string)
	for key, val := range sourceLabels {
		agentLabels[key] = val
	}
	agentLabels[infrav1.AgentControllerLabel] = agentGroup
	return agentLabels
}

func agentsInGroup(ctx context.Context, ctrlclient client.Client, agentGroup *infrav1.KubeforceAgentGroup) ([]*infrav1.KubeforceAgent, error) {
	list := &infrav1.KubeforceAgentList{}
	listOpts := []client.ListOption{
		client.InNamespace(agentGroup.Namespace),
		client.MatchingLabels(map[string]string{
			infrav1.AgentControllerLabel: agentGroup.Name,
		}),
	}
	err := ctrlclient.List(ctx, list, listOpts...)
	if err != nil {
		return nil, err
	}
	agents := make([]*infrav1.KubeforceAgent, 0)
	for _, kfAgent := range list.Items {
		//nolint:gosec
		if metav1.IsControlledBy(&kfAgent, agentGroup) {
			agents = append(agents, kfAgent.DeepCopy())
		}
	}
	return agents, nil
}

func agentsBySelector(ctx context.Context, ctrlclient client.Client, namespace string, selector labels.Selector) ([]*infrav1.KubeforceAgent, error) {
	list := &infrav1.KubeforceAgentList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}

	err := ctrlclient.List(ctx, list, listOpts...)
	if err != nil {
		return nil, err
	}
	agents := make([]*infrav1.KubeforceAgent, 0)
	for _, kfAgent := range list.Items {
		agents = append(agents, kfAgent.DeepCopy())
	}
	return agents, nil
}

func (r *KubeforceAgentGroupReconciler) desiredAgents(agentGroup *infrav1.KubeforceAgentGroup) []*infrav1.KubeforceAgent {
	agents := make([]*infrav1.KubeforceAgent, 0, len(agentGroup.Spec.Addresses))
	agentLabels := buildKubeforceAgentLabels(agentGroup.Spec.Template.ObjectMeta.Labels, agentGroup.Name)
	for key, address := range agentGroup.Spec.Addresses {
		suffix := fmt.Sprintf("-%s", key)
		spec := agentGroup.Spec.Template.Spec.DeepCopy()
		spec.Addresses = address.DeepCopy()

		m := &infrav1.KubeforceAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:        names.BuildName(agentGroup.Name, suffix),
				Namespace:   agentGroup.Namespace,
				Labels:      agentLabels,
				Annotations: agentGroup.Spec.Template.ObjectMeta.Annotations,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(agentGroup, infrav1.GroupVersion.WithKind("KubeforceAgentGroup")),
				},
			},
			Spec: *spec,
		}
		agents = append(agents, m)
	}
	return agents
}

func (r *KubeforceAgentGroupReconciler) updateAgent(ctx context.Context, agent *infrav1.KubeforceAgent, desiredAgent *infrav1.KubeforceAgent) error {
	// Calculate patch data.
	patchObj := client.MergeFrom(agent.DeepCopy())

	for key, value := range desiredAgent.Labels {
		agent.Labels[key] = value
	}

	// only SSH and CertIssuerRef fields can be changed
	agent.Spec.SSH = desiredAgent.Spec.SSH
	agent.Spec.Config.CertTemplate = desiredAgent.Spec.Config.CertTemplate

	changed, err := patchutil.HasChanges(patchObj, agent)
	if err != nil {
		return errors.WithStack(err)
	}

	if changed {
		if err := r.Client.Patch(ctx, agent, patchObj); err != nil {
			return errors.Wrapf(err, "failed to patch KubeforceAgent")
		}
	}
	return nil
}

func patchKubeforceAgentGroup(ctx context.Context, patchHelper *patch.Helper, agentGroup *infrav1.KubeforceAgentGroup) error {
	conditions.SetSummary(agentGroup,
		conditions.WithConditions(
			clusterv1.InfrastructureReadyCondition,
		),
		conditions.WithStepCounterIf(agentGroup.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		agentGroup,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			clusterv1.InfrastructureReadyCondition,
		}},
	)
}

func agentsToMap(agents []*infrav1.KubeforceAgent) map[string]*infrav1.KubeforceAgent {
	m := make(map[string]*infrav1.KubeforceAgent, len(agents))
	for _, agent := range agents {
		m[agent.Name] = agent
	}
	return m
}
