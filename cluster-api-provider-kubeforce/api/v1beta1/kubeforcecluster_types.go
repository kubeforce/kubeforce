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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows KubeforceClusterReconciler to clean up resources associated with KubeforceCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "kubeforcecluster.infrastructure.cluster.x-k8s.io"
)

// KubeforceClusterSpec defines the desired state of KubeforceCluster
type KubeforceClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// KubeforceClusterStatus defines the observed state of KubeforceCluster
type KubeforceClusterStatus struct {
	// Ready denotes that the cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// APIServers describes the kube-apiserver addresses for configuring the loadbalancer on the nodes.
	APIServers []string `json:"apiServers,omitempty"`

	// FailureDomains is a slice of FailureDomains.
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// Conditions defines current service state of the KubeforceCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=kubeforceclusters,scope=Namespaced,shortName=kfc
// +kubebuilder:subresource:status

// KubeforceCluster is the Schema for the kubeforceclusters API
type KubeforceCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceClusterSpec   `json:"spec,omitempty"`
	Status KubeforceClusterStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (c *KubeforceCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *KubeforceCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// KubeforceClusterList contains a list of KubeforceCluster
type KubeforceClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceCluster{}, &KubeforceClusterList{})
}
