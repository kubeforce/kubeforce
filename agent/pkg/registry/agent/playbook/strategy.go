/*
Copyright 2021 The Kubeforce Authors.

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

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"

	"k3f.io/kubeforce/agent/pkg/apis/agent"
	"k3f.io/kubeforce/agent/pkg/apis/agent/validation"
)

// NewStrategy creates and returns a playbookStrategy instance.
func NewStrategy(typer runtime.ObjectTyper) playbookStrategy {
	return playbookStrategy{typer, names.SimpleNameGenerator}
}

// GetAttrs returns labels.Set, fields.Set, and error in case the given runtime.Object is not a Fischer.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	apiserver, ok := obj.(*agent.Playbook)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Playbook")
	}
	return apiserver.ObjectMeta.GetLabels(), SelectableFields(apiserver), nil
}

// MatchPlaybook is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func MatchPlaybook(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that represents the object.
func SelectableFields(obj *agent.Playbook) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

var _ rest.RESTCreateStrategy = playbookStrategy{}
var _ rest.RESTUpdateStrategy = playbookStrategy{}
var _ rest.RESTDeleteStrategy = playbookStrategy{}

type playbookStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (s playbookStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// WarningsOnCreate returns warnings to the client performing a create.
func (s playbookStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// NamespaceScoped returns true if the object must be within a namespace.
func (playbookStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate is invoked on create before validation to normalize the object.
func (playbookStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	pb := obj.(*agent.Playbook)
	pb.Status = agent.PlaybookStatus{}

	pb.Generation = 1
}

// PrepareForUpdate is invoked on update before validation to normalize the object.
func (playbookStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newObj := obj.(*agent.Playbook)
	oldObj := old.(*agent.Playbook)
	newObj.Status = oldObj.Status
}

// Validate validates a new playbook.
func (playbookStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	pb := obj.(*agent.Playbook)
	return validation.ValidatePlaybookCreate(pb)
}

// AllowCreateOnUpdate is false for playbooks.
func (playbookStrategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate allows playbooks to be overwritten.
func (playbookStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize allows an object to be mutated into a canonical form.
func (playbookStrategy) Canonicalize(obj runtime.Object) {
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (playbookStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}

	return fields
}

// ValidateUpdate is the default update validation for an end user.
func (playbookStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	newObj := obj.(*agent.Playbook)
	oldObj := old.(*agent.Playbook)
	return validation.ValidatePlaybookUpdate(newObj, oldObj)
}

var _ rest.RESTCreateStrategy = playbookStatusStrategy{}
var _ rest.RESTUpdateStrategy = playbookStatusStrategy{}
var _ rest.RESTDeleteStrategy = playbookStatusStrategy{}

type playbookStatusStrategy struct {
	playbookStrategy
}

// NewStatusStrategy creates and returns a playbookStatusStrategy instance.
func NewStatusStrategy(typer runtime.ObjectTyper) playbookStatusStrategy {
	return playbookStatusStrategy{NewStrategy(typer)}
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (playbookStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

// PrepareForUpdate is invoked on update before validation to normalize the object status.
func (playbookStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newObj := obj.(*agent.Playbook)
	oldObj := old.(*agent.Playbook)
	newObj.Spec = oldObj.Spec
}
