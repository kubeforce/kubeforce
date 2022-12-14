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

package playbook

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
)

// TemplateReconciler manages playbooks for PlaybookControlObject.
type TemplateReconciler struct {
	Client client.Client
	Log    logr.Logger
}

// Reconcile reconciles playbooks controlled by the PlaybookControlObject.
func (r *TemplateReconciler) Reconcile(ctx context.Context, obj infrav1.PlaybookControlObject, templateType infrav1.TemplateType, vars map[string]interface{}) (bool, error) {
	if obj.GetTemplates() == nil {
		return true, nil
	}
	// to avoid duplication, new playbooks should be added to the status immediately
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := patchHelper.Patch(ctx, obj); err != nil {
			r.Log.
				WithValues("name", obj.GetName(), "kind", obj.GetObjectKind()).
				Error(err, "failed to patch object")
		}
	}()
	var condType clusterv1.ConditionType
	switch templateType {
	case infrav1.TemplateTypeInstall:
		condType = infrav1.InitPlaybooksCondition
	case infrav1.TemplateTypeDelete:
		condType = infrav1.CleanupPlaybooksCondition
	default:
		return false, errors.Errorf("unsupported templateType %q", templateType)
	}
	references := r.getReferences(obj.GetTemplates().References, templateType)
	vars = r.mergeVars(vars, obj.GetTemplates().Variables)
	for _, ref := range references {
		ready, err := r.reconcileReference(ctx, obj, ref, vars)
		if err != nil {
			conditions.MarkFalse(obj, condType, infrav1.PlaybooksDeployingFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return false, err
		}
		if !ready {
			msg := fmt.Sprintf("waiting for playbook with role: %s", ref.role)
			conditions.MarkFalse(obj, condType, infrav1.WaitingForCompletionPhaseReason, clusterv1.ConditionSeverityInfo, msg)
			return false, nil
		}
	}
	conditions.MarkTrue(obj, condType)
	return true, err
}

func (r *TemplateReconciler) mergeVars(vars1 map[string]interface{}, vars2 map[string]runtime.RawExtension) map[string]interface{} {
	if len(vars2) == 0 {
		return vars1
	}
	result := make(map[string]interface{})
	for k, v := range vars1 {
		result[k] = v
	}
	for k, v := range vars2 {
		result[k] = v
	}
	return result
}

func (r *TemplateReconciler) reconcileReference(ctx context.Context, obj infrav1.PlaybookControlObject, ref reference, vars map[string]interface{}) (bool, error) {
	switch ref.ref.Kind {
	case "PlaybookTemplate":
		return r.reconcilePlaybook(ctx, obj, ref, vars)
	case "PlaybookDeploymentTemplate":
		return r.reconcilePlaybookDeployment(ctx, obj, ref, vars)
	default:
		return false, errors.Errorf("unsupported kind %q", ref.ref.Kind)
	}
}

func (r *TemplateReconciler) getReferences(templates map[string]*infrav1.TemplateReference, templateType infrav1.TemplateType) []reference {
	refs := make([]reference, 0)
	for role, ref := range templates {
		if ref.Type != templateType {
			continue
		}
		refs = append(refs, reference{
			role: role,
			ref:  *ref,
		})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].ref.Priority > refs[j].ref.Priority
	})
	return refs
}

func (r *TemplateReconciler) getPlaybookTemplate(ctx context.Context, ref infrav1.TemplateReference) (*infrav1.PlaybookTemplate, error) {
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	template := &infrav1.PlaybookTemplate{}
	err := r.Client.Get(ctx, key, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (r *TemplateReconciler) getPlaybookDeploymentTemplate(ctx context.Context, ref infrav1.TemplateReference) (*infrav1.PlaybookDeploymentTemplate, error) {
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	template := &infrav1.PlaybookDeploymentTemplate{}
	err := r.Client.Get(ctx, key, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (r *TemplateReconciler) reconcilePlaybookDeployment(ctx context.Context, obj infrav1.PlaybookControlObject, ref reference, vars map[string]interface{}) (bool, error) {
	role := ref.role
	pd, err := r.findPlaybookDeploymentByRole(ctx, obj, role)
	if err != nil {
		return false, err
	}
	playbookConditions := obj.GetPlaybookConditions()
	if playbookConditions == nil {
		playbookConditions = make(infrav1.PlaybookConditions)
	}
	template, err := r.getPlaybookDeploymentTemplate(ctx, ref.ref)
	if err != nil {
		return false, err
	}
	if pd != nil {
		playbookConditions[role] = &infrav1.PlaybookCondition{
			Ref:   objToRef(pd),
			Phase: pd.Status.ExternalPhase,
		}
		obj.SetPlaybookConditions(playbookConditions)
		updated, err := r.updatePlaybookDeployment(ctx, obj, pd, template, role, vars)
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
	pd, err = r.createPlaybookDeployment(ctx, obj, template, role, vars)
	if err != nil {
		return false, err
	}
	playbookConditions[role] = &infrav1.PlaybookCondition{
		Ref:   objToRef(pd),
		Phase: pd.Status.ExternalPhase,
	}
	obj.SetPlaybookConditions(playbookConditions)
	return false, nil
}

func (r *TemplateReconciler) reconcilePlaybook(ctx context.Context, obj infrav1.PlaybookControlObject, ref reference, vars map[string]interface{}) (bool, error) {
	role := ref.role
	playbook, err := r.findPlaybookByRole(ctx, obj, role)
	if err != nil {
		return false, err
	}
	playbookConditions := obj.GetPlaybookConditions()
	if playbookConditions == nil {
		playbookConditions = make(infrav1.PlaybookConditions)
	}
	template, err := r.getPlaybookTemplate(ctx, ref.ref)
	if err != nil {
		return false, err
	}
	if playbook != nil {
		playbookConditions[role] = &infrav1.PlaybookCondition{
			Ref:   objToRef(playbook),
			Phase: playbook.Status.ExternalPhase,
		}
		obj.SetPlaybookConditions(playbookConditions)
		if conditions.IsTrue(playbook, infrav1.SynchronizationCondition) && playbook.Status.ExternalPhase == "Succeeded" {
			return true, nil
		}
		return false, nil
	}
	playbook, err = r.createPlaybook(ctx, obj, template, role, vars)
	if err != nil {
		return false, err
	}
	playbookConditions[role] = &infrav1.PlaybookCondition{
		Ref:   objToRef(playbook),
		Phase: playbook.Status.ExternalPhase,
	}
	obj.SetPlaybookConditions(playbookConditions)
	return false, nil
}

func createLabels(obj infrav1.PlaybookControlObject, role string) map[string]string {
	return map[string]string{
		clusterv1.ClusterLabelName:              obj.GetLabels()[clusterv1.ClusterLabelName],
		infrav1.PlaybookRoleLabelName:           role,
		infrav1.PlaybookAgentNameLabelName:      obj.GetAgent().Name,
		infrav1.PlaybookControllerNameLabelName: obj.GetName(),
		infrav1.PlaybookControllerKindLabelName: obj.GetObjectKind().GroupVersionKind().GroupKind().String(),
	}
}

func (r *TemplateReconciler) findPlaybookByRole(ctx context.Context, obj infrav1.PlaybookControlObject, role string) (*infrav1.Playbook, error) {
	list := &infrav1.PlaybookList{}
	listOptions := client.MatchingLabelsSelector{
		Selector: labels.Set(createLabels(obj, role)).AsSelector(),
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

func (r *TemplateReconciler) findPlaybookDeploymentByRole(ctx context.Context, obj infrav1.PlaybookControlObject, role string) (*infrav1.PlaybookDeployment, error) {
	list := &infrav1.PlaybookDeploymentList{}
	listOptions := client.MatchingLabelsSelector{
		Selector: labels.Set(createLabels(obj, role)).AsSelector(),
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

func (r *TemplateReconciler) updatePlaybookDeployment(ctx context.Context, obj infrav1.PlaybookControlObject, pd *infrav1.PlaybookDeployment, tmpl *infrav1.PlaybookDeploymentTemplate, role string, vars map[string]interface{}) (bool, error) {
	patchObj := client.MergeFrom(pd.DeepCopy())
	for key, value := range createLabels(obj, role) {
		pd.Labels[key] = value
	}
	pd.Spec.AgentRef = corev1.LocalObjectReference{
		Name: obj.GetAgent().Name,
	}
	pd.Spec.Template.Spec = infrav1.RemotePlaybookSpec{
		Files:      tmpl.Spec.Template.Spec.Files,
		Entrypoint: tmpl.Spec.Template.Spec.Entrypoint,
	}
	if vars != nil {
		varsData, err := yaml.Marshal(vars)
		if err != nil {
			return false, errors.Wrapf(err, "unable to marshal variables for PlaybookDeployment %s", pd.Name)
		}
		pd.Spec.Template.Spec.Files["variables.yaml"] = string(varsData)
	}

	changed, err := patchutil.HasChanges(patchObj, pd)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if changed {
		r.Log.Info("updating PlaybookDeployment", "key", client.ObjectKeyFromObject(pd))
		err := r.Client.Patch(ctx, pd, patchObj)
		if err != nil {
			return false, errors.Wrapf(err, "failed to patch PlaybookDeployment")
		}
		return true, nil
	}
	return false, nil
}

func (r *TemplateReconciler) createPlaybookDeployment(ctx context.Context, obj infrav1.PlaybookControlObject, tmpl *infrav1.PlaybookDeploymentTemplate, role string, vars map[string]interface{}) (*infrav1.PlaybookDeployment, error) {
	suffix := fmt.Sprintf("-%s-", role)
	pd := &infrav1.PlaybookDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(obj.GetName() + suffix),
			Namespace: obj.GetNamespace(),
			Labels:    createLabels(obj, role),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1.GroupVersion.String(),
					Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
					Name:               obj.GetName(),
					UID:                obj.GetUID(),
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: infrav1.PlaybookDeploymentSpec{
			AgentRef: corev1.LocalObjectReference{
				Name: obj.GetAgent().Name,
			},
			Template: infrav1.PlaybookTemplateSpec{
				Spec: infrav1.RemotePlaybookSpec{
					Files:      tmpl.Spec.Template.Spec.Files,
					Entrypoint: tmpl.Spec.Template.Spec.Entrypoint,
				},
			},
			Paused: false,
		},
	}
	if len(vars) > 0 {
		varsData, err := yaml.Marshal(vars)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to marshal variables for PlaybookDeployment %s", pd.Name)
		}
		pd.Spec.Template.Spec.Files["variables.yaml"] = string(varsData)
	}
	r.Log.Info("creating PlaybookDeployment", "key", client.ObjectKeyFromObject(pd))
	err := r.Client.Create(ctx, pd)
	if err != nil {
		return nil, err
	}
	return pd, nil
}

func (r *TemplateReconciler) createPlaybook(ctx context.Context, obj infrav1.PlaybookControlObject, tmpl *infrav1.PlaybookTemplate, role string, vars map[string]interface{}) (*infrav1.Playbook, error) {
	suffix := fmt.Sprintf("-%s-", role)
	p := &infrav1.Playbook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName(obj.GetName() + suffix),
			Namespace: obj.GetNamespace(),
			Labels:    createLabels(obj, role),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1.GroupVersion.String(),
					Kind:               obj.GetObjectKind().GroupVersionKind().Kind,
					Name:               obj.GetName(),
					UID:                obj.GetUID(),
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: infrav1.PlaybookSpec{
			AgentRef: corev1.LocalObjectReference{
				Name: obj.GetAgent().Name,
			},
			RemotePlaybookSpec: infrav1.RemotePlaybookSpec{
				Files:      tmpl.Spec.Spec.Files,
				Entrypoint: tmpl.Spec.Spec.Entrypoint,
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

type reference struct {
	role string
	ref  infrav1.TemplateReference
}

// objToRef returns a reference to the given object.
func objToRef(obj client.Object) *corev1.ObjectReference {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return &corev1.ObjectReference{
		Kind:       gvk.Kind,
		APIVersion: gvk.GroupVersion().String(),
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
	}
}
