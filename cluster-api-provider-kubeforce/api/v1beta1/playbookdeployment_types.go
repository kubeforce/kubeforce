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
	// PlaybookDeploymentFinalizer allows PlaybookDeploymentReconciler to clean up resources associated
	// with PlaybookDeployment before removing it from the apiserver.
	PlaybookDeploymentFinalizer = "playbookdeployment.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=playbookdeployments,scope=Namespaced,shortName=pbd
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Agent",type="string",JSONPath=".spec.agentRef.name",description="KubeforceAgent"
// +kubebuilder:printcolumn:name="ExternalPhase",type="string",JSONPath=".status.externalPhase"
// +kubebuilder:printcolumn:name="ExternalName",type="string",JSONPath=".status.externalName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// PlaybookDeployment is the Schema for the playbookdeployments API.
type PlaybookDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlaybookDeploymentSpec   `json:"spec,omitempty"`
	Status PlaybookDeploymentStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (in *PlaybookDeployment) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *PlaybookDeployment) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// PlaybookDeploymentList contains a list of PlaybookDeployment.
type PlaybookDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlaybookDeployment `json:"items"`
}

// PlaybookDeploymentSpec defines the desired state of PlaybookDeployment.
type PlaybookDeploymentSpec struct {
	// AgentRef is a reference to the agent
	AgentRef corev1.LocalObjectReference `json:"agentRef"`
	// Template describes the playbook that will be created.
	Template PlaybookTemplateSpec `json:"template"`
	// The number of old Playbook to retain for history.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// PlaybookDeploymentStatus defines the observed state of PlaybookDeployment.
type PlaybookDeploymentStatus struct {
	// ExternalName is the name of PlaybookDeployment on the node
	// +optional
	ExternalName string `json:"externalName,omitempty"`

	// ExternalPhase is the phase of a PlaybookDeployment, high-level summary of where the PlaybookDeployment is in its lifecycle.
	// +optional
	ExternalPhase string `json:"externalPhase,omitempty"`

	// ExternalObservedGeneration is the latest generation observed in an external PlaybookDeployment.
	// +optional
	ExternalObservedGeneration int64 `json:"externalObservedGeneration,omitempty"`

	// Conditions defines current service state of the PlaybookDeployment.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func init() {
	SchemeBuilder.Register(&PlaybookDeployment{}, &PlaybookDeploymentList{})
}
