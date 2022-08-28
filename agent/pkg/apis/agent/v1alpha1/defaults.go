package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_Policy assigns default values for the execution policy
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
//nolint:stylecheck,revive
func SetDefaults_PlaybookDeploymentSpec(obj *PlaybookDeploymentSpec) {
	if obj.RevisionHistoryLimit == nil {
		limit := int32(10)
		obj.RevisionHistoryLimit = &limit
	}
}

// SetDefaults_PlaybookSpec assigns default values for the PlaybookSpec
//nolint:stylecheck,revive
func SetDefaults_PlaybookSpec(obj *PlaybookSpec) {
	if obj.Policy == nil {
		obj.Policy = &Policy{}
	}
}
