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

package validation

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k3f.io/kubeforce/agent/pkg/apis/agent"
)

// ValidatePlaybookCreate validates a playbook in the context of its initial create.
func ValidatePlaybookCreate(obj *agent.Playbook) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(&obj.ObjectMeta, false, apimachineryvalidation.NameIsDNSSubdomain, field.NewPath("metadata"))
	allErrs = append(allErrs, validatePlaybookSpec(&obj.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePlaybookSpec(p *agent.PlaybookSpec, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if p.Policy != nil {
		allErrs = append(allErrs, validatePolicy(p.Policy, field.NewPath("policy"))...)
	}
	if len(p.Files) == 0 {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("files"), p.Files, "cannot be empty"))
	}
	if p.Entrypoint == "" {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("entrypoint"), p.Entrypoint, "cannot be empty"))
	}
	return allErrs
}

func validatePolicy(p *agent.Policy, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if p.BackoffLimit != nil {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*p.BackoffLimit), fieldPath.Child("backoffLimit"))...)
	}
	if p.Timeout != nil {
		allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(int64(*p.BackoffLimit), fieldPath.Child("timeout"))...)
	}
	return allErrs
}

// ValidatePlaybookUpdate tests to see if the update is legal. The agent.Playbook is an immutable object.
func ValidatePlaybookUpdate(newObj *agent.Playbook, oldObj *agent.Playbook) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(&newObj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))
	if !cmp.Equal(newObj.Spec, oldObj.Spec) {
		specDiff := cmp.Diff(newObj.Spec, oldObj.Spec)
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), fmt.Sprintf("playbook is immutable. diff: \n%s", specDiff)))
	}
	return allErrs
}

// ValidatePlaybookLogOptions tests if the options for getting logs are legal.
func ValidatePlaybookLogOptions(opts *agent.PlaybookLogOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	if opts.TailLines != nil && *opts.TailLines < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("tailLines"), *opts.TailLines, apimachineryvalidation.IsNegativeErrorMsg))
	}
	switch {
	case opts.SinceSeconds != nil && opts.SinceTime != nil:
		allErrs = append(allErrs, field.Forbidden(field.NewPath(""), "at most one of `sinceTime` or `sinceSeconds` may be specified"))
	case opts.SinceSeconds != nil:
		if *opts.SinceSeconds < 1 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("sinceSeconds"), *opts.SinceSeconds, "must be greater than 0"))
		}
	}
	return allErrs
}

// ValidatePlaybookDeploymentCreate validates a PlaybookDeployment in the context of its initial create.
func ValidatePlaybookDeploymentCreate(obj *agent.PlaybookDeployment) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(&obj.ObjectMeta, false, apimachineryvalidation.NameIsDNSSubdomain, field.NewPath("metadata"))
	allErrs = append(allErrs, validatePlaybookSpec(&obj.Spec.Template.Spec, field.NewPath("spec", "template", "spec"))...)
	return allErrs
}

// ValidatePlaybookDeploymentUpdate tests to see if the update is legal. The agent.PlaybookDeployment is an immutable object.
func ValidatePlaybookDeploymentUpdate(newObj *agent.PlaybookDeployment, oldObj *agent.PlaybookDeployment) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(&newObj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))
	return allErrs
}
