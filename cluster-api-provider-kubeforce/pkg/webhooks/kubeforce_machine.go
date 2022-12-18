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

package webhooks

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/assets"
	utiltmpl "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/templates"
)

// SetupWebhookWithManager sets up KubeforceMachine webhooks.
func (webhook *KubeforceMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.KubeforceMachine{}).
		WithDefaulter(webhook).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-kubeforcemachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachines,versions=v1beta1,name=validation.kubeforcemachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-kubeforcemachine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=kubeforcemachines,versions=v1beta1,name=default.kubeforcemachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// KubeforceMachine implements a validating webhook for KubeforceMachine.
type KubeforceMachine struct {
	Client client.Reader
}

var _ webhook.CustomDefaulter = &KubeforceMachine{}
var _ webhook.CustomValidator = &KubeforceMachine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *KubeforceMachine) Default(ctx context.Context, obj runtime.Object) error {
	kfm, ok := obj.(*infrav1.KubeforceMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KubeforceMachine but got a %T", obj))
	}
	if kfm.Spec.PlaybookTemplates == nil {
		kfm.Spec.PlaybookTemplates = &infrav1.PlaybookTemplates{}
	}
	if kfm.Spec.PlaybookTemplates.References == nil {
		kfm.Spec.PlaybookTemplates.References = make(map[string]*infrav1.TemplateReference)
	}
	kfc, err := webhook.findKubeforceCluster(ctx, kfm)
	if err != nil {
		return err
	}
	webhook.defaultTemplateReferences(kfm.Spec.PlaybookTemplates.References, kfc)
	return nil
}

func (webhook *KubeforceMachine) findKubeforceCluster(ctx context.Context, kfm *infrav1.KubeforceMachine) (*infrav1.KubeforceCluster, error) {
	if kfm.Labels[clusterv1.ClusterLabelName] == "" {
		return nil, nil
	}
	clusterKey := client.ObjectKey{
		Namespace: kfm.Namespace,
		Name:      kfm.Labels[clusterv1.ClusterLabelName],
	}
	cluster := &clusterv1.Cluster{}
	if err := webhook.Client.Get(ctx, clusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to get Cluster %s", clusterKey))
	}

	kubeforceCluster := &infrav1.KubeforceCluster{}
	kfcKey := client.ObjectKey{
		Namespace: kfm.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := webhook.Client.Get(ctx, kfcKey, kubeforceCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, apierrors.NewBadRequest(fmt.Sprintf("unable to get KubeforceCluster %s", kfcKey))
	}
	return kubeforceCluster, nil
}

func (webhook *KubeforceMachine) defaultTemplateReferences(refs map[string]*infrav1.TemplateReference, kfc *infrav1.KubeforceCluster) {
	initRole := string(assets.PlaybookInstaller)
	if refs[initRole] == nil {
		refs[initRole] = &infrav1.TemplateReference{
			Kind:       "PlaybookTemplate",
			Namespace:  infrav1.KubeforceSystemNamespace,
			Name:       utiltmpl.GetName(assets.PlaybookInstaller),
			APIVersion: infrav1.GroupVersion.String(),
		}
	}
	refs[initRole].Priority = 1000
	refs[initRole].Type = infrav1.TemplateTypeInstall
	if kfc != nil && (kfc.Spec.Loadbalancer == nil || !kfc.Spec.Loadbalancer.Disabled) {
		lbRole := string(assets.PlaybookLoadbalancer)
		if refs[lbRole] == nil {
			refs[lbRole] = &infrav1.TemplateReference{
				Kind:       "PlaybookDeploymentTemplate",
				Namespace:  infrav1.KubeforceSystemNamespace,
				Name:       utiltmpl.GetName(assets.PlaybookLoadbalancer),
				APIVersion: infrav1.GroupVersion.String(),
			}
		}
		refs[lbRole].Priority = 100
		refs[lbRole].Type = infrav1.TemplateTypeInstall
	}

	cleanerRole := string(assets.PlaybookCleaner)
	if refs[cleanerRole] == nil {
		refs[cleanerRole] = &infrav1.TemplateReference{
			Kind:       "PlaybookTemplate",
			Namespace:  infrav1.KubeforceSystemNamespace,
			Name:       utiltmpl.GetName(assets.PlaybookCleaner),
			APIVersion: infrav1.GroupVersion.String(),
		}
	}

	refs[cleanerRole].Priority = 1000
	refs[cleanerRole].Type = infrav1.TemplateTypeDelete
}

// ValidateCreate implements webhook.CustomValidator.
func (webhook *KubeforceMachine) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	m, ok := obj.(*infrav1.KubeforceMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an KubeforceMachine but got a %T", obj))
	}
	return webhook.validate(ctx, m)
}

// ValidateUpdate implements webhook.CustomValidator.
func (webhook *KubeforceMachine) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldMachine, ok := oldObj.(*infrav1.KubeforceMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an KubeforceMachine but got a %T", oldObj))
	}
	newMachine, ok := newObj.(*infrav1.KubeforceMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an KubeforceMachine but got a %T", newObj))
	}
	if oldMachine.Spec.AgentRef != nil && !reflect.DeepEqual(oldMachine.Spec.AgentRef, newMachine.Spec.AgentRef) {
		return field.Forbidden(
			field.NewPath("spec", "agentRef", "name"),
			"the AgentRef of KubeforceMachine is immutable. It cannot be changed if it is initialized",
		)
	}
	return webhook.validate(ctx, newMachine)
}

// ValidateDelete implements webhook.CustomValidator.
func (webhook *KubeforceMachine) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (webhook *KubeforceMachine) validate(ctx context.Context, m *infrav1.KubeforceMachine) error {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	if m.Spec.PlaybookTemplates == nil {
		return nil
	}

	for key, ref := range m.Spec.PlaybookTemplates.References {
		refPath := specPath.Child("playbookTemplates", "references", key)
		if err := webhook.validateTemplateReferences(ctx, refPath, ref); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return allErrs.ToAggregate()
}

func (webhook *KubeforceMachine) validateTemplateReferences(ctx context.Context, path *field.Path, ref *infrav1.TemplateReference) *field.Error {
	if ref.Priority <= 0 {
		return field.Invalid(
			path.Child("priority"),
			ref.Priority,
			"value must be greater than zero",
		)
	}
	if ref.Type != infrav1.TemplateTypeInstall && ref.Type != infrav1.TemplateTypeDelete {
		return field.NotSupported(
			path.Child("type"),
			ref.Type,
			[]string{infrav1.TemplateTypeInstall, infrav1.TemplateTypeDelete},
		)
	}
	if ref.APIVersion != infrav1.GroupVersion.String() {
		return field.NotSupported(
			path.Child("apiVersion"),
			ref.APIVersion,
			[]string{infrav1.GroupVersion.String()},
		)
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	switch ref.Kind {
	case "PlaybookTemplate":
		if _, err := webhook.getPlaybookTemplate(ctx, key); err != nil {
			return field.Invalid(path, key, "unable to get PlaybookTemplate")
		}
	case "PlaybookDeploymentTemplate":
		if _, err := webhook.getPlaybookDeploymentTemplate(ctx, key); err != nil {
			return field.Invalid(path, key, "unable to get PlaybookDeploymentTemplate")
		}
	default:
		return field.NotSupported(path.Child("kind"), ref.Kind, []string{"PlaybookTemplate", "PlaybookDeploymentTemplate"})
	}
	return nil
}

func (webhook *KubeforceMachine) getPlaybookTemplate(ctx context.Context, key client.ObjectKey) (*infrav1.PlaybookTemplate, error) {
	template := &infrav1.PlaybookTemplate{}
	err := webhook.Client.Get(ctx, key, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (webhook *KubeforceMachine) getPlaybookDeploymentTemplate(ctx context.Context, key client.ObjectKey) (*infrav1.PlaybookDeploymentTemplate, error) {
	template := &infrav1.PlaybookDeploymentTemplate{}
	err := webhook.Client.Get(ctx, key, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}
