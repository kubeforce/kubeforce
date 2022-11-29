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

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=playbookdeploymenttemplates,scope=Namespaced,shortName=pbdt
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// PlaybookDeploymentTemplate is the Schema for the playbook templates API.
type PlaybookDeploymentTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PlaybookDeploymentTemplateSpec `json:"spec,omitempty"`
}

// PlaybookDeploymentTemplateSpec describes the data a playbook should have when created from a template.
type PlaybookDeploymentTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Template describes the playbook that will be created.
	Template PlaybookTemplateSpec `json:"template"`

	// The number of old Playbook to retain for history.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
}

//+kubebuilder:object:root=true

// PlaybookDeploymentTemplateList contains a list of PlaybookDeploymentTemplate.
type PlaybookDeploymentTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlaybookDeploymentTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlaybookDeploymentTemplate{}, &PlaybookDeploymentTemplateList{})
}
