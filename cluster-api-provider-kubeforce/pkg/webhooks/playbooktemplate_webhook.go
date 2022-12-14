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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

// SetupWebhookWithManager sets up the controller with the Manager.
func (webhook *PlaybookTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.PlaybookTemplate{}).
		WithValidator(webhook).
		Complete()
}

// PlaybookTemplate implements a validating webhook for PlaybookTemplate.
type PlaybookTemplate struct {
	Client client.Reader
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-playbooktemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=playbooktemplates,verbs=delete,versions=v1beta1,name=vplaybooktemplate.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &PlaybookTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *PlaybookTemplate) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *PlaybookTemplate) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *PlaybookTemplate) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	t, ok := obj.(*infrav1.PlaybookTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an PlaybookTemplate but got a %T", obj))
	}
	return webhook.validateDelete(ctx, t)
}

func (webhook *PlaybookTemplate) validateDelete(ctx context.Context, t *infrav1.PlaybookTemplate) error {
	list := &infrav1.KubeforceMachineList{}
	err := webhook.Client.List(ctx, list)
	if err != nil {
		return err
	}
	for _, ma := range list.Items {
		if ma.Spec.PlaybookTemplates == nil {
			continue
		}
		for _, ref := range ma.Spec.PlaybookTemplates.References {
			if ref.Kind == "PlaybookTemplate" && ref.Name == t.Name && ref.Namespace == t.Namespace {
				return apierrors.NewForbidden(infrav1.GroupVersion.WithResource("PlaybookTemplate").GroupResource(), t.Name,
					fmt.Errorf("PlaybookTemplate cannot be deleted because it is used by KubeforceMachine %s", client.ObjectKeyFromObject(t)))
			}
		}
	}
	return nil
}
