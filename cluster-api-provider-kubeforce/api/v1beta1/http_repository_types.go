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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HTTPRepositoryFinalizer allows HTTPRepositoryReconcile to clean up resources associated with HTTPRepository before
	// removing it from the apiserver.
	HTTPRepositoryFinalizer = "repository.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:resource:path=httprepositories,scope=Namespaced,shortName=httprepo
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.url`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceAgent"

// HTTPRepository is the Schema for the httprepositories API
type HTTPRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HTTPRepositorySpec `json:"spec,omitempty"`
}

// HTTPRepositorySpec specifies the configuration for connecting to a http repository.
type HTTPRepositorySpec struct {
	// URL specifies the url of repository
	// +required
	URL string `json:"url"`

	// Insecure allows connecting to a non-TLS HTTP Endpoint.
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// SecretRef specifies the Secret containing authentication credentials
	// for the Repository.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// Timeout for fetch operations, defaults to 60s.
	// +kubebuilder:default="60s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

//+kubebuilder:object:root=true

// HTTPRepositoryList contains a list of HTTPRepository
type HTTPRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HTTPRepository{}, &HTTPRepositoryList{})
}
