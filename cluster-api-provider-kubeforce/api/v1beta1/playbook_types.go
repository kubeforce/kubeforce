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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// PlaybookFinalizer allows PlaybookReconciler to clean up resources associated with Playbook before
	// removing it from the apiserver.
	PlaybookFinalizer = "playbook.infrastructure.cluster.x-k8s.io"

	// PlaybookRoleLabelName is the role of the playbook.
	PlaybookRoleLabelName = "playbook.infrastructure.cluster.x-k8s.io/role"

	// PlaybookControllerNameLabelName is a name of the playbook controller.
	PlaybookControllerNameLabelName = "playbook.infrastructure.cluster.x-k8s.io/controller-name"

	// PlaybookControllerKindLabelName is a group and a kind of the playbook controller.
	// format: <group>.<kind>
	PlaybookControllerKindLabelName = "playbook.infrastructure.cluster.x-k8s.io/controller-kind"

	// PlaybookAgentNameLabelName is a name of the agent.
	PlaybookAgentNameLabelName = "playbook.infrastructure.cluster.x-k8s.io/agent"
)

// PlaybookSpec defines the desired state of Playbook.
type PlaybookSpec struct {
	RemotePlaybookSpec `json:",inline"`
	// AgentRef is a reference to the agent
	AgentRef corev1.LocalObjectReference `json:"agentRef"`
}

// RemotePlaybookSpec describes the remote Playbook in the agent.
type RemotePlaybookSpec struct {
	Files      map[string]string `json:"files,omitempty"`
	Entrypoint string            `json:"entrypoint,omitempty"`
}

// PlaybookStatus defines the observed state of Playbook.
type PlaybookStatus struct {
	// ExternalName is the name of playbook on the node
	// +optional
	ExternalName string `json:"externalName,omitempty"`

	// ExternalPhase is the phase of a Playbook, high-level summary of where the Playbook is in its lifecycle.
	// +optional
	ExternalPhase string `json:"externalPhase,omitempty"`

	// Ready denotes that the playbook is completed
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the Playbook.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=playbooks,scope=Namespaced,shortName=pb
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Agent",type="string",JSONPath=".spec.agentRef.name",description="KubeforceAgent"
// +kubebuilder:printcolumn:name="ExternalPhase",type="string",JSONPath=".status.externalPhase"
// +kubebuilder:printcolumn:name="ExternalName",type="string",JSONPath=".status.externalName"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// Playbook is the Schema for the playbooks API.
type Playbook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlaybookSpec   `json:"spec,omitempty"`
	Status PlaybookStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (in *Playbook) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *Playbook) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// PlaybookList contains a list of Playbook.
type PlaybookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Playbook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Playbook{}, &PlaybookList{})
}
