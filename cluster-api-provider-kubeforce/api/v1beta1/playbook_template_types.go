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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PlaybookTemplateFinalizer allows PlaybookTemplateReconciler to clean up resources associated with PlaybookTemplate before
	// removing it from the apiserver.
	PlaybookTemplateFinalizer = "playbook.template.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=playbooktemplates,scope=Namespaced,shortName=pbt
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// PlaybookTemplate is the Schema for the playbook templates API.
type PlaybookTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PlaybookTemplateSpec `json:"spec,omitempty"`
}

// PlaybookTemplateSpec describes the data a playbook should have when created from a template.
type PlaybookTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the playbook.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec RemotePlaybookSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// PlaybookTemplateList contains a list of PlaybookTemplate.
type PlaybookTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlaybookTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlaybookTemplate{}, &PlaybookTemplateList{})
}
