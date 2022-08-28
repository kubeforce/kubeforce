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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// AgentFinalizer allows KubeforceAgentReconcile to clean up resources associated with KubeforceAgent before
	// removing it from the apiserver.
	AgentFinalizer = "kubeforceagent.infrastructure.cluster.x-k8s.io"

	// AgentControllerLabel is a name of the agent group controller
	AgentControllerLabel = "kubeforceagent.infrastructure.cluster.x-k8s.io/group-name"

	// AgentMachineLabel is a name of the KubeforceMachine using this agent.
	AgentMachineLabel = "kubeforceagent.infrastructure.cluster.x-k8s.io/machine"
)

// +kubebuilder:resource:path=kubeforceagents,scope=Namespaced,shortName=kfa
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".metadata.labels['kubeforceagent\\.infrastructure\\.cluster\\.x-k8s\\.io/group-name']",description="KubeforceAgentGroup"
// +kubebuilder:printcolumn:name="KfMachine",type="string",JSONPath=".metadata.labels['kubeforceagent\\.infrastructure\\.cluster\\.x-k8s\\.io/machine']",description="KubeforceMachine"
// +kubebuilder:printcolumn:name="ExternalIP",type="string",JSONPath=".spec.addresses.externalIP",description="External IP address of KubeforceAgent"
// +kubebuilder:printcolumn:name="ExternalDNS",type="string",JSONPath=".spec.addresses.externalDNS",description="External DNS address of KubeforceAgent"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="KubeforceAgent phase"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.conditions[?(@.type==\"Healthy\")].status",description="Health status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceAgent"

// KubeforceAgent is the Schema for the kubeforceagents API
type KubeforceAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceAgentSpec   `json:"spec,omitempty"`
	Status KubeforceAgentStatus `json:"status,omitempty"`
}

// KubeforceAgentSpec defines the desired state of KubeforceAgent
type KubeforceAgentSpec struct {
	// Installed is true when the agent has been installed on the host
	// +optional
	Installed bool `json:"installed,omitempty"`

	// Source is a source of the agent binary.
	// +optional
	Source *AgentSource `json:"source,omitempty"`

	// Addresses is a list of addresses assigned to the host where agent is installed.
	// +optional
	Addresses *Addresses `json:"addresses,omitempty"`

	// System
	// +kubebuilder:default:={arch: "amd64", os:"linux"}
	// +optional
	System SystemParams `json:"system,omitempty"`

	//SSH
	// +optional
	SSH SSHParams `json:"ssh,omitempty"`

	// Config is an agent configuration
	// +optional
	Config AgentConfigSpec `json:"config"`
}

type AgentSource struct {
	// RepoRef specifies a repository of the agent.
	// +kubebuilder:default:={kind: "HTTPRepository", name:"github", namespace: "kubeforce-system"}
	// +optional
	RepoRef *RepositoryReference `json:"repoRef,omitempty"`
	// Path to the directory containing the agent file.
	// The default is empty, which translates to the root path of the RepositoryReference.
	// +optional
	Path string `json:"path,omitempty"`
	// Version specifies a version of agent. The controller version is used by default.
	// +optional
	Version string `json:"version,omitempty"`
}

// AgentConfigSpec is an agent configuration
type AgentConfigSpec struct {
	// CertIssuerRef is a reference to the issuer for agent certificates.
	// If the `kind` field is not set, or set to `Issuer`, an Issuer resource
	// with the given name in the same namespace as the KubeforceCluster will be used.
	// If the `kind` field is set to `ClusterIssuer`, a ClusterIssuer with the
	// provided name will be used.
	// The `name` field in this stanza is required at all times.
	CertIssuerRef corev1.TypedLocalObjectReference `json:"certIssuerRef"`
	// Authentication specifies how requests to the Agent's server are authenticated
	// +optional
	Authentication AgentAuthentication `json:"authentication"`
}

// AgentAuthentication is configuration for agent authrntication
type AgentAuthentication struct {
	// X509 contains settings related to x509 client certificate authentication
	// +optional
	X509 AgentX509Authentication `json:"x509"`
}

// AgentX509Authentication is ...
type AgentX509Authentication struct {
	CaSecret     string `json:"caSecret"`
	ClientSecret string `json:"clientSecret"`
}

type SystemParams struct {
	// Arch is GOARCH for current node.
	// +kubebuilder:default="amd64"
	// +optional
	Arch string `json:"arch,omitempty"`
	// Arch is GOOS for current node.
	// +kubebuilder:default="linux"
	// +optional
	Os string `json:"os,omitempty"`
}

type SSHParams struct {
	// Port is the port for ssh connection.
	// Default: 22
	// +optional
	Port       int    `json:"port,omitempty"`
	Username   string `json:"username,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

// Addresses is addresses assigned to the node.
type Addresses struct {
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	ExternalIP string `json:"externalIP,omitempty"`
	// +optional
	InternalIP string `json:"internalIP,omitempty"`
	// +optional
	ExternalDNS string `json:"externalDNS,omitempty"`
	// +optional
	InternalDNS string `json:"internalDNS,omitempty"`
}

// KubeforceAgentStatus defines the observed state of KubeforceAgent
type KubeforceAgentStatus struct {
	// Phase represents the current phase of agent actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase AgentPhase `json:"phase,omitempty"`

	// Conditions defines current service state of the KubeforceAgent.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// FailureReason will be set in case of a terminal problem machine
	// and will contain a short value suitable for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Agent's spec or the configuration of
	// the controller, and that manual intervention is required.
	// +optional
	FailureReason AgentStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in case of a terminal problem
	// reconciling and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// AgentInfo is information that describes the installed agent.
	AgentInfo *AgentInfo `json:"agentInfo,omitempty"`
}

// AgentInfo is information that describes the installed agent
type AgentInfo struct {
	// Version reported by the agent.
	Version string `json:"version"`
	// GitCommit is the git commit hash reported by the agent
	GitCommit string `json:"gitCommit"`
	// The platform reported by the agent
	Platform string `json:"platform"`
	// The build date reported by the agent
	BuildDate string `json:"buildDate"`
}

var _ conditions.Setter = &KubeforceAgent{}

// GetConditions returns the set of conditions for this object.
func (in *KubeforceAgent) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (in *KubeforceAgent) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// KubeforceAgentList contains a list of KubeforceAgent
type KubeforceAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceAgent{}, &KubeforceAgentList{})
}
