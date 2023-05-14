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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
// +kubebuilder:printcolumn:name="Agent",type="string",JSONPath=".status.agentRef.name",description="KubeforceAgent"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="KubeforceMachine ready state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceMachine"

// KubeforceMachine is the Schema for the kubeforcemachines API.
type KubeforceMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceMachineSpec   `json:"spec,omitempty"`
	Status KubeforceMachineStatus `json:"status,omitempty"`
}

// GetAgent returns the agent reference.
func (in *KubeforceMachine) GetAgent() types.NamespacedName {
	return types.NamespacedName{
		Namespace: in.Namespace,
		Name:      in.Status.AgentRef.Name,
	}
}

// GetTemplates returns the map of TemplateReferences.
func (in *KubeforceMachine) GetTemplates() *PlaybookTemplates {
	return in.Spec.PlaybookTemplates
}

// GetPlaybookConditions returns conditions of playbooks.
func (in *KubeforceMachine) GetPlaybookConditions() PlaybookConditions {
	return in.Status.Playbooks
}

// SetPlaybookConditions save conditions of playbooks to the managed object.
func (in *KubeforceMachine) SetPlaybookConditions(playbookConditions PlaybookConditions) {
	in.Status.Playbooks = playbookConditions
}

// KubeforceMachineSpec defines the desired state of KubeforceMachine.
type KubeforceMachineSpec struct {
	// ProviderID will be the container name in ProviderID format (kf://<cluster>-<machine>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Label selector for agents. If agentRef is empty controller
	// will find free agent by this selector and update agentRef field.
	AgentSelector *metav1.LabelSelector `json:"agentSelector,omitempty"`

	// PlaybookTemplates describes playbookTemplates that are managed by the KubeforceMachine.
	PlaybookTemplates *PlaybookTemplates `json:"playbookTemplates,omitempty"`
}

// PlaybookTemplates is a set of references to a PlaybookTemplate or PlaybookDeploymentTemplate.
type PlaybookTemplates struct {
	// References are references to PlaybookTemplate or PlaybookDeploymentTemplate that are managed.
	// KubeforceMachine has predifined roles "init", "loadblanacer", "cleanup".
	// If these predefined TemplateReferences have not been specified by users, they will be created automatically.
	// +optional
	References map[string]*TemplateReference `json:"refs,omitempty"`

	// Variables are additional variables that are used to create the Playbook and PlaybookDeployment.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Variables map[string]runtime.RawExtension `json:"variables,omitempty"`
}

// TemplateReference is the reference to the PlaybookTemplate or PlaybookDeploymentTemplate.
// Playbook or PlaybookDeployment is created from these templates during the KubeforceMachine lifecycle.
type TemplateReference struct {
	// Kind of the referent.
	// +kubebuilder:validation:Enum=PlaybookTemplate;PlaybookDeploymentTemplate
	Kind string `json:"kind,omitempty"`
	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name,omitempty"`
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`
	// The priority value.
	// The higher the value, the higher the priority.
	Priority int32 `json:"priority,omitempty"`
	// Type indicates in which phase of the KubeforceMachine life cycle this template will be executed.
	// +optional
	// +kubebuilder:validation:Enum=install;delete
	Type TemplateType `json:"type,omitempty"`
}

// TemplateType indicates in which phase of the Object life cycle this template will be executed.
type TemplateType string

const (
	// TemplateTypeInstall creates Playbooks from this template during object initialization.
	TemplateTypeInstall = "install"
	// TemplateTypeDelete generated Playbooks from this template when the object is deleted.
	TemplateTypeDelete = "delete"
)

// KubeforceMachineStatus defines the observed state of KubeforceMachine.
type KubeforceMachineStatus struct {
	// Ready denotes that the machine is ready
	// +optional
	Ready bool `json:"ready"`

	// AgentRef is a reference to the agent resource that holds the details
	// for provisioning infrastructure for a cluster.
	// +optional
	AgentRef *corev1.LocalObjectReference `json:"agentRef,omitempty"`

	// Playbooks are playbooks that are controlled by KubeforceMachine.
	// +optional
	Playbooks PlaybookConditions `json:"playbooks,omitempty"`

	// DefaultIPAddress is an ip address from default route.
	DefaultIPAddress string `json:"defaultIPAddress,omitempty"`

	// Conditions defines current service state of the KubeforceMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// PlaybookConditions provide observations of the operational state of a managed Playbook.
type PlaybookConditions map[string]*PlaybookCondition

// PlaybookCondition defines current service state of the managed Playbook.
type PlaybookCondition struct {
	// Ref will point to the corresponding Playbook or PlaybookDeployment.
	Ref *corev1.ObjectReference `json:"ref,omitempty"`
	// Phase is the phase of a Playbook, high-level summary of where the Playbook is in its lifecycle.
	// +optional
	Phase string `json:"externalPhase,omitempty"`
	// Last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed. If that is not known, then using the time when
	// the API field changed is acceptable.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// PlaybookInfo describes the high-level summary of controlled playbooks.
type PlaybookInfo struct {
	// Name is a name of playbook
	Name string `json:"name"`
	// Phase is the phase of a Playbook, high-level summary of where the Playbook is in its lifecycle.
	// +optional
	Phase string `json:"externalPhase,omitempty"`
}

var _ conditions.Setter = &KubeforceMachine{}

// GetConditions returns the set of conditions for this object.
func (in *KubeforceMachine) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *KubeforceMachine) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

var _ PlaybookControlObject = &KubeforceMachine{}

//+kubebuilder:object:root=true

// KubeforceMachineList contains a list of KubeforceMachine.
type KubeforceMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceMachine{}, &KubeforceMachineList{})
}
