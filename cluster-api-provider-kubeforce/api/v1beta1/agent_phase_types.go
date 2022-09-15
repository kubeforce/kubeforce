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

// AgentPhase is a string representation of a KubeforceAgent Phase.
//
// Controllers should always look at the actual state of the KubeforceAgent’s fields to make those decisions.
type AgentPhase string

const (
	// AgentPhasePending is the first state an Agent is assigned by.
	AgentPhasePending AgentPhase = "Pending"

	// AgentPhaseProvisioning is the state when the
	// Agent infrastructure is being created.
	AgentPhaseProvisioning AgentPhase = "Provisioning"

	// AgentPhaseInstalled is the state when agent has been installed and configured.
	AgentPhaseInstalled AgentPhase = "Installed"

	// AgentPhaseRunning is the Agent state when it has
	// become a Kubernetes Node in a Ready state.
	AgentPhaseRunning AgentPhase = "Running"

	// AgentPhaseDeleting is the Agent state when a delete
	// request has been sent to the API Server,
	// but its infrastructure has not yet been fully deleted.
	AgentPhaseDeleting AgentPhase = "Deleting"

	// AgentPhaseFailed is the Agent state when the system
	// might require user intervention.
	AgentPhaseFailed AgentPhase = "Failed"

	// AgentPhaseUnknown is returned if the Agent state cannot be determined.
	AgentPhaseUnknown AgentPhase = "Unknown"
)
