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

// PlaybookStatusError defines errors states for Playbook and PlaybookDeployment objects.
type PlaybookStatusError string

const (
	// AgentIsNotReadyPlaybookError indicates an error connecting to the agent.
	//
	// This may indicate a transient issue that will be fixed automatically resolved with time,
	// such as a node failure or the bastion host is unavailable.
	AgentIsNotReadyPlaybookError PlaybookStatusError = "AgentIsNotReady"

	// AgentClientPlaybookError indicates an error while trying to get the ClientSet for the agent
	// where this Playbook should be executed.
	AgentClientPlaybookError PlaybookStatusError = "AgentClientError"

	// ExternalPlaybookError indicates an error while trying to get external Playbook.
	ExternalPlaybookError PlaybookStatusError = "ExternalPlaybookError"

	// DeletePlaybookError indicates an error was encountered while trying to delete the Playbook.
	DeletePlaybookError PlaybookStatusError = "DeleteError"
)
