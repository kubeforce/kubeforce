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
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// MachineFinalizer allows ReconcileKubeforceMachine to clean up resources associated with KubeforceMachine before
	// removing it from the apiserver.
	MachineFinalizer = "kubeforcemachine.infrastructure.cluster.x-k8s.io"

	// KubeforceClusterLabelName is the label set on machines or related objects that points to a KubeforceCluster.
	KubeforceClusterLabelName = "infrastructure.cluster.x-k8s.io/kubeforce-cluster-name"
)

// +kubebuilder:resource:path=kubeforcemachines,scope=Namespaced,shortName=kfm
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Agent",type="string",JSONPath=".spec.agentRef.name",description="KubeforceAgent"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="KubeforceMachine ready state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceMachine"

// KubeforceMachine is the Schema for the kubeforcemachines API
type KubeforceMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceMachineSpec   `json:"spec,omitempty"`
	Status KubeforceMachineStatus `json:"status,omitempty"`
}

// KubeforceMachineSpec defines the desired state of KubeforceMachine
type KubeforceMachineSpec struct {
	// ProviderID will be the container name in ProviderID format (kf://<cluster>-<machine>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// AgentRef is a reference to the agent resource that holds the details
	// for provisioning infrastructure for a cluster.
	// +optional
	AgentRef *corev1.LocalObjectReference `json:"agentRef,omitempty"`

	// Label selector for agents. If agentRef is empty controller
	// will find free agent by this selector and update agentRef field.
	AgentSelector *metav1.LabelSelector `json:"agentSelector,omitempty"`
}

// KubeforceMachineStatus defines the observed state of KubeforceMachine
type KubeforceMachineStatus struct {
	// Ready denotes that the machine is ready
	// +optional
	Ready bool `json:"ready"`

	// Playbooks are playbooks that are controlled by KubeforceMachine.
	// +optional
	Playbooks map[string]*PlaybookRefs `json:"playbooks,omitempty"`

	// InternalIP is an ip address from default interface
	InternalIP string `json:"internalIP,omitempty"`

	// Conditions defines current service state of the KubeforceMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type PlaybookRefs struct {
	// Name is a name of playbook
	Name string `json:"name"`
	// Phase is the phase of a Playbook, high-level summary of where the Playbook is in its lifecycle.
	// +optional
	Phase string `json:"externalPhase,omitempty"`
}

var _ conditions.Setter = &KubeforceMachine{}

// GetConditions returns the set of conditions for this object.
func (c *KubeforceMachine) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *KubeforceMachine) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// KubeforceMachineList contains a list of KubeforceMachine
type KubeforceMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceMachine{}, &KubeforceMachineList{})
}
