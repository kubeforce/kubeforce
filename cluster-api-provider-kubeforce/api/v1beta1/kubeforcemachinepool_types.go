/*
Copyright 2021.

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

const (
	// MachinePoolFinalizer allows KubeforceMachinePoolReconciler to clean up resources.
	MachinePoolFinalizer = "kubeforcemachinepool.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:resource:path=kubeforcemachinepools,scope=Namespaced,categories=cluster-api,shortName=kfmp
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.replicas",description="MachinePool replicas count"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="KubeforceMachinePool ready state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceMachinePool"

// KubeforceMachinePool is the Schema for the kubeforcemachinepools API.
type KubeforceMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceMachinePoolSpec   `json:"spec,omitempty"`
	Status KubeforceMachinePoolStatus `json:"status,omitempty"`
}

// KubeforceMachinePoolSpec defines the desired state of KubeforceMachinePool.
type KubeforceMachinePoolSpec struct {
	// Template contains the details used to build a replica machine within the Machine Pool
	// +optional
	Template KubeforceMachineTemplateResource `json:"template"`

	// ProviderID is the identification ID of the Machine Pool
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// ProviderIDList is the list of identification IDs of machine instances managed by this Machine Pool
	//+optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

// KubeforceMachinePoolStatus defines the observed state of KubeforceMachinePool.
type KubeforceMachinePoolStatus struct {
	// Ready denotes that the machine pool is ready
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas"`

	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the KubeforceMachinePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (c *KubeforceMachinePool) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *KubeforceMachinePool) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// KubeforceMachinePoolList contains a list of KubeforceMachinePool.
type KubeforceMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceMachinePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceMachinePool{}, &KubeforceMachinePoolList{})
}
