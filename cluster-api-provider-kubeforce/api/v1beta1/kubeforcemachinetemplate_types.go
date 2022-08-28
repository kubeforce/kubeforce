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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// KubeforceMachineTemplateSpec defines the desired state of KubeforceMachineTemplate.
type KubeforceMachineTemplateSpec struct {
	Template KubeforceMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=kubeforcemachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceMachinePool"

// KubeforceMachineTemplate is the Schema for the kubeforcemachinetemplates API.
type KubeforceMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KubeforceMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KubeforceMachineTemplateList contains a list of KubeforceMachineTemplate.
type KubeforceMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceMachineTemplate{}, &KubeforceMachineTemplateList{})
}

// KubeforceMachineTemplateResource describes the data needed to create a KubeforceMachine from a template.
type KubeforceMachineTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the specification of the desired behavior of the machine.
	Spec KubeforceMachineSpec `json:"spec"`
}
