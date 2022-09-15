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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Playbook is the Schema for the playbooks API.
// +k8s:openapi-gen=true
type Playbook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlaybookSpec   `json:"spec,omitempty"`
	Status PlaybookStatus `json:"status,omitempty"`
}

// PlaybookList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlaybookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Playbook `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (in *Playbook) GetConditions() Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *Playbook) SetConditions(conditions Conditions) {
	in.Status.Conditions = conditions
}

// PlaybookSpec defines the desired state of Playbook.
type PlaybookSpec struct {
	// Policy is the playbook execution policy
	// +optional
	Policy *Policy `json:"policy,omitempty"`
	// Files is playbook files.
	// The key is a file name, the value of the map is the content of the file.
	Files map[string]string `json:"files"`
	// Entrypoint is file path to execute this playbook.
	// Entrypoint must be one of file specified in the Files field of this playbook
	Entrypoint string `json:"entrypoint"`
}

// PlaybookStatus defines the observed state of Playbook.
type PlaybookStatus struct {
	// Phase is the phase of a Playbook, high-level summary of where the Playbook is in its lifecycle.
	// +optional
	Phase PlaybookPhase `json:"phase,omitempty"`
	// Conditions defines current state of the Playbook.
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`
	// The number of times the playbook has reached the Failed phase.
	// +optional
	Failed int32 `json:"failed,omitempty"`
}

// Policy defines the playbook execution policy
type Policy struct {
	// Specifies the duration in seconds relative to the startTime that the job may be active
	// before the system tries to terminate it.
	// Defaults to 10m
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Specifies the number of retries before marking this playbook failed.
	// Defaults to 3
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

// PlaybookPhase defines the phase of playbook at the current time.
type PlaybookPhase string

// These are the valid phases of Playbook.
const (
	// PlaybookPending means the Playbook has been accepted by the system, but it has not been started.
	PlaybookPending PlaybookPhase = "Pending"
	// PlaybookRunning means the Playbook has been started.
	PlaybookRunning PlaybookPhase = "Running"
	// PlaybookSucceeded means that the Playbook has terminated with an exit code of 0,
	// and the system is not going to restart this Playbook.
	PlaybookSucceeded PlaybookPhase = "Succeeded"
	// PlaybookFailed means that the Playbook has terminated, in a failure
	// (exited with a non-zero exit code or was stopped by the system).
	PlaybookFailed PlaybookPhase = "Failed"
	// PlaybookUnknown means that for some reason the state of the Playbook could not be obtained.
	PlaybookUnknown PlaybookPhase = "Unknown"
)

// Conditions provide observations of the operational state.
type Conditions []Condition

// ConditionType is a valid value for Condition.Type.
type ConditionType string

// Condition defines an observation of a Cluster API resource operational state.
type Condition struct {
	// Type of condition in CamelCase or in foo.example.com/CamelCase.
	// Many .condition.type values are consistent across resources like Available, but because arbitrary conditions
	// can be useful (see .node.status.conditions), the ability to deconflict is important.
	Type ConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// Last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed. If that is not known, then using the time when
	// the API field changed is acceptable.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// The reason for the condition's last transition in CamelCase.
	// The specific API may choose whether or not this field is considered a guaranteed API.
	// This field may not be empty.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the transition.
	// This field may be empty.
	// +optional
	Message string `json:"message,omitempty"`
}
