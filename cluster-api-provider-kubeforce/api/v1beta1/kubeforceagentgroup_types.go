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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// AgentGroupFinalizer allows KubeforceAgentGroupReconcile to clean up resources associated with KubeforceAgentGroup before
	// removing it from the apiserver.
	AgentGroupFinalizer = "kubeforceagentgroup.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:resource:path=kubeforceagentgroups,scope=Namespaced,shortName=kfag
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Replicas"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="KubeforceAgentGroup ready state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceAgentGroup"

// KubeforceAgentGroup is the Schema for the kubeforceagentgroups API.
type KubeforceAgentGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceAgentGroupSpec   `json:"spec,omitempty"`
	Status KubeforceAgentGroupStatus `json:"status,omitempty"`
}

// KubeforceAgentGroupSpec defines the desired state of KubeforceAgentGroup.
type KubeforceAgentGroupSpec struct {
	// Addresses is addresses assigned to the agents created from this group.
	// +optional
	Addresses map[string]Addresses `json:"addresses,omitempty"`

	// Template defines the agents that will be created from this kubeforce agent template.
	// +optional
	Template KubeforceAgentTemplateSpec `json:"template,omitempty"`
}

// KubeforceAgentTemplateSpec describes the data a KubeforceAgent should have when created from a template.
type KubeforceAgentTemplateSpec struct {
	// Standard object's metadata.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the KubeforceAgent.
	// +optional
	Spec KubeforceAgentSpec `json:"spec,omitempty"`
}

// KubeforceAgentGroupStatus defines the observed state of KubeforceAgentGroup.
type KubeforceAgentGroupStatus struct {
	// Ready denotes that the agent is ready
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the KubeforceAgent.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas"`

	// ReadyReplicas is the number of agents in the ready state.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

var _ conditions.Setter = &KubeforceAgentGroup{}

// GetConditions returns the set of conditions for this object.
func (in *KubeforceAgentGroup) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *KubeforceAgentGroup) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// KubeforceAgentGroupList contains a list of KubeforceAgentGroup.
type KubeforceAgentGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceAgentGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceAgentGroup{}, &KubeforceAgentGroupList{})
}
