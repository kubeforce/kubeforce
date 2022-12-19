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

// PlaybookPhase is a string representation of a Playbook and PlaybookDeployment Phase.
type PlaybookPhase string

const (
	// PlaybookPhaseProvisioning is the state when the
	// remote Playbook is being created.
	PlaybookPhaseProvisioning PlaybookPhase = "Provisioning"

	// PlaybookPhaseSynchronization is the state of the Playbook when the controller is synchronized with the remote object.
	PlaybookPhaseSynchronization PlaybookPhase = "Synchronization"

	// PlaybookPhaseCompleted is the state when the synchronization phase is completed
	// and the Playbook has successfully completed.
	PlaybookPhaseCompleted PlaybookPhase = "Completed"

	// PlaybookPhaseDeleting is the Playbook state when a delete
	// request has been sent to the API Server, but the remote object has not yet been fully deleted.
	PlaybookPhaseDeleting PlaybookPhase = "Deleting"

	// PlaybookPhaseFailed is the Playbook state when the system
	// might require user intervention.
	PlaybookPhaseFailed PlaybookPhase = "Failed"

	// PlaybookPhaseUnknown is returned if the Playbook state cannot be determined.
	PlaybookPhaseUnknown PlaybookPhase = "Unknown"
)
