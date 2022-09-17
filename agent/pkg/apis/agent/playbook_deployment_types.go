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

package agent

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlaybookDeployment provides declarative updates for Playbook.
type PlaybookDeployment struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Specification of the desired behavior of the Deployment.
	Spec PlaybookDeploymentSpec
	// Most recently observed status of the Deployment.
	// +optional
	Status PlaybookDeploymentStatus
}

// PlaybookDeploymentList defines multiple deployments.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlaybookDeploymentList struct {
	metav1.TypeMeta
	metav1.ListMeta
	// Items is the list of deployments.
	Items []PlaybookDeployment
}

// PlaybookTemplateSpec describes the data a playbook should have when created from a template.
type PlaybookTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta

	// Specification of the desired behavior of the playbook.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec PlaybookSpec
}

// PlaybookDeploymentSpec defines the desired state of PlaybookDeployment.
type PlaybookDeploymentSpec struct {
	// Template describes the playbook that will be created.
	Template PlaybookTemplateSpec
	// The number of old Playbook to retain for history.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32
	// Indicates that the deployment is paused.
	// +optional
	Paused bool
}

// PlaybookDeploymentStatus defines the observed state of PlaybookDeployment.
type PlaybookDeploymentStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64
	// Phase is the phase of a PlaybookDeployment, high-level summary of where the PlaybookDeployment is in its lifecycle.
	// +optional
	Phase PlaybookDeploymentPhase
}

// PlaybookDeploymentPhase defines the phase of PlaybookDeployment at the current time.
type PlaybookDeploymentPhase string

// These are the valid phases of PlaybookDeployment.
const (
	// PlaybookDeploymentProgressing means the PlaybookDeployment is progressing.
	PlaybookDeploymentProgressing PlaybookDeploymentPhase = "Progressing"
	// PlaybookDeploymentSucceeded means that the last Playbook completed successfully.
	PlaybookDeploymentSucceeded PlaybookDeploymentPhase = "Succeeded"
	// PlaybookDeploymentPaused means that the PlaybookDeployment is paused.
	PlaybookDeploymentPaused PlaybookDeploymentPhase = "Paused"
	// PlaybookDeploymentFailed means that the last Playbook did not complete successfully.
	PlaybookDeploymentFailed PlaybookDeploymentPhase = "Failed"
)
