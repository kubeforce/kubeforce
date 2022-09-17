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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_Policy assigns default values for the execution policy.
//
//nolint:stylecheck,revive
func SetDefaults_Policy(obj *Policy) {
	if obj.Timeout == nil {
		obj.Timeout = &metav1.Duration{Duration: 10 * time.Minute}
	}
	if obj.BackoffLimit == nil {
		limit := int32(3)
		obj.BackoffLimit = &limit
	}
}

// SetDefaults_PlaybookDeploymentSpec assigns default values for the PlaybookDeploymentSpec
//
//nolint:stylecheck,revive
func SetDefaults_PlaybookDeploymentSpec(obj *PlaybookDeploymentSpec) {
	if obj.RevisionHistoryLimit == nil {
		limit := int32(10)
		obj.RevisionHistoryLimit = &limit
	}
}

// SetDefaults_PlaybookSpec assigns default values for the PlaybookSpec
//
//nolint:stylecheck,revive
func SetDefaults_PlaybookSpec(obj *PlaybookSpec) {
	if obj.Policy == nil {
		obj.Policy = &Policy{}
	}
}
