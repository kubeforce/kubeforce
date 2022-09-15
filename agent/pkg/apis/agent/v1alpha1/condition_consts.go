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

// Conditions and Reasons for the Playbook object.
const (
	// PlaybookExecutionCondition documents the execution status of the playbook.
	PlaybookExecutionCondition ConditionType = "Execution"

	// PlaybookFailedCondition means the playbook has failed its execution.
	PlaybookFailedCondition ConditionType = "Failed"

	// DeadlineExceededReason means that playbook was active longer than specified timeout.
	DeadlineExceededReason = "DeadlineExceeded"

	// BackoffLimitExceededReason means that playbook has reached the specified deferral limit.
	BackoffLimitExceededReason = "BackoffLimitExceeded"

	// PlaybookExecutionFailedReason documents a Playbook detecting
	// an error while execution the playbook.
	PlaybookExecutionFailedReason = "ExecutionFailed"

	// PlaybookPreparationFailedReason documents a Playbook when an error occurs during prepare phase.
	PlaybookPreparationFailedReason = "PreparationFailed"
)
