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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// AgentFinalizer allows KubeforceAgentReconcile to clean up resources associated with KubeforceAgent before
	// removing it from the apiserver.
	AgentFinalizer = "kubeforceagent.infrastructure.cluster.x-k8s.io"

	// AgentControllerLabel is a name of the agent group controller.
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
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="Ready status"
// +kubebuilder:printcolumn:name="Health",type="string",JSONPath=".status.conditions[?(@.type==\"Healthy\")].status",description="Health status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KubeforceAgent"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.agentInfo.version",description="Installed agent version"
// +kubebuilder:printcolumn:name="Platform",type="string",priority=1,JSONPath=".status.agentInfo.platform",description="Platform architecture information"

// KubeforceAgent is the Schema for the kubeforceagents API.
type KubeforceAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeforceAgentSpec   `json:"spec,omitempty"`
	Status KubeforceAgentStatus `json:"status,omitempty"`
}

// KubeforceAgentSpec defines the desired state of KubeforceAgent.
type KubeforceAgentSpec struct {
	// Installed is true when the agent has been installed on the host
	// +optional
	Installed bool `json:"installed,omitempty"`

	// Source is a source of the agent binary.
	// +kubebuilder:default:={repoRef:{kind: "HTTPRepository", name:"github", namespace: "kubeforce-system"}}
	// +optional
	Source *AgentSource `json:"source,omitempty"`

	// Addresses is a list of addresses assigned to the host where agent is installed.
	// +optional
	Addresses *Addresses `json:"addresses,omitempty"`

	// System is the system params for the host.
	// +kubebuilder:default:={arch: "amd64", os:"linux"}
	// +optional
	System SystemParams `json:"system,omitempty"`

	// SSH is a params for ssh connection.
	// +optional
	SSH SSHParams `json:"ssh,omitempty"`

	// Config is an agent configuration
	// +optional
	Config AgentConfigSpec `json:"config"`
}

// AgentSource describes the source from where the agent will be installed and its version.
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

// AgentConfigSpec is an agent configuration.
type AgentConfigSpec struct {
	// CertTemplate is a template for agent certificate.
	CertTemplate CertificateTemplate `json:"certTemplate"`
	// Authentication specifies how requests to the Agent's server are authenticated
	// +optional
	Authentication AgentAuthentication `json:"authentication"`
}

// CertificateTemplate is a template for Certificate object.
type CertificateTemplate struct {
	// IssuerRef is a reference to the issuer for this certificate.
	IssuerRef CertObjectReference `json:"issuerRef"`

	// The requested 'duration' (i.e. lifetime) of the Certificate.
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// How long before the currently issued certificate's expiry
	// cert-manager should renew the certificate.
	// +optional
	RenewBefore *metav1.Duration `json:"renewBefore,omitempty"`

	// DNSNames is a list of DNS subjectAltNames to be set on the Certificate.
	// +optional
	DNSNames []string `json:"dnsNames,omitempty"`

	// IPAddresses is a list of IP address subjectAltNames to be set on the Certificate.
	// +optional
	IPAddresses []string `json:"ipAddresses,omitempty"`

	// Options to control private keys used for the Certificate.
	// +optional
	PrivateKey *CertificatePrivateKey `json:"privateKey,omitempty"`
}

// CertObjectReference is a reference to an object with a given name, kind and group.
type CertObjectReference struct {
	// Name of the resource being referred to.
	Name string `json:"name"`
	// Kind of the resource being referred to.
	// +optional
	Kind string `json:"kind,omitempty"`
	// Group of the resource being referred to.
	// +optional
	Group string `json:"group,omitempty"`
}

// CertificatePrivateKey contains configuration options for private keys
// used by the Certificate controller.
// This allows control of how private keys are rotated.
type CertificatePrivateKey struct {
	// RotationPolicy controls how private keys should be regenerated when a
	// re-issuance is being processed.
	// +optional
	// +kubebuilder:validation:Enum=Never;Always
	RotationPolicy string `json:"rotationPolicy,omitempty"`

	// The private key cryptography standards (PKCS) encoding for this
	// certificate's private key to be encoded in.
	// If provided, allowed values are `PKCS1` and `PKCS8` standing for PKCS#1
	// and PKCS#8, respectively.
	// Defaults to `PKCS1` if not specified.
	// +optional
	// +kubebuilder:validation:Enum=PKCS1;PKCS8
	Encoding string `json:"encoding,omitempty"`

	// Algorithm is the private key algorithm of the corresponding private key
	// for this certificate. If provided, allowed values are either `RSA`,`Ed25519` or `ECDSA`
	// If `algorithm` is specified and `size` is not provided,
	// key size of 256 will be used for `ECDSA` key algorithm and
	// key size of 2048 will be used for `RSA` key algorithm.
	// key size is ignored when using the `Ed25519` key algorithm.
	// +optional
	// +kubebuilder:validation:Enum=RSA;ECDSA;Ed25519
	Algorithm string `json:"algorithm,omitempty"`

	// Size is the key bit size of the corresponding private key for this certificate.
	// If `algorithm` is set to `RSA`, valid values are `2048`, `4096` or `8192`,
	// and will default to `2048` if not specified.
	// If `algorithm` is set to `ECDSA`, valid values are `256`, `384` or `521`,
	// and will default to `256` if not specified.
	// If `algorithm` is set to `Ed25519`, Size is ignored.
	// No other values are allowed.
	// +optional
	Size int `json:"size,omitempty"`
}

// AgentAuthentication is configuration for agent authentication.
type AgentAuthentication struct {
	// X509 contains settings related to x509 client certificate authentication
	// +optional
	X509 AgentX509Authentication `json:"x509"`
}

// AgentX509Authentication describes configuration of x509 client certificate authentication.
type AgentX509Authentication struct {
	// ClientSecret is the name of the secret in the same namespace as the KubeforceAgent.
	ClientSecret string `json:"clientSecret"`
}

// SystemParams describes the system of the host.
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

// SSHParams describes the parameters for connecting via ssh.
type SSHParams struct {
	// Port is the port for ssh connection.
	// +kubebuilder:default=22
	// +optional
	Port int `json:"port,omitempty"`
	// Username is a name of user to connect via ssh.
	Username string `json:"username,omitempty"`
	// SecretName is the name of the secret that stores the password or private ssh key.
	SecretName string `json:"secretName,omitempty"`
}

// Addresses is addresses assigned to the node.
type Addresses struct {
	// +optional
	ExternalIP string `json:"externalIP,omitempty"`
	// +optional
	InternalIP string `json:"internalIP,omitempty"`
	// +optional
	ExternalDNS string `json:"externalDNS,omitempty"`
	// +optional
	InternalDNS string `json:"internalDNS,omitempty"`
}

// KubeforceAgentStatus defines the observed state of KubeforceAgent.
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

	// AgentInfo is information that describes the installed agent.
	// +optional
	SystemInfo *SystemInfo `json:"systemInfo,omitempty"`
}

// AgentInfo is information that describes the installed agent.
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

// SystemInfo defines system information from the host.
type SystemInfo struct {
	// Network is the network information
	// +optional
	Network NetworkInfo `json:"network"`
}

// NetworkInfo defines the network information.
type NetworkInfo struct {
	// Hostname is the current hostname
	// +optional
	Hostname string `json:"hostname"`
	// DefaultIPAddress is an ip address from default route
	// +optional
	DefaultIPAddress string `json:"defaultIPAddress"`
	// DefaultInterfaceName is a network interface from default route
	// +optional
	DefaultInterfaceName string `json:"defaultInterfaceName"`
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

// KubeforceAgentList contains a list of KubeforceAgent.
type KubeforceAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeforceAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeforceAgent{}, &KubeforceAgentList{})
}
