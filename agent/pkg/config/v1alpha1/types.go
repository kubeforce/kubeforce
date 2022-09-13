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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Config defines agent configuration
type Config struct {
	metav1.TypeMeta `json:",inline"`
	Spec            ConfigSpec `json:"spec"`
}

// ConfigSpec defines agent configuration
type ConfigSpec struct {
	// Port is the port for the Agent to serve on.
	Port int `json:"port"`
	// TLS specifies tls configuration for the server.
	TLS TLS `json:"tls"`
	// authentication specifies how requests to the Agent's server are authenticated
	Authentication AgentAuthentication `json:"authentication"`
	// ShutdownGracePeriod specifies the total grace period  for shutdown the server.
	// +optional
	ShutdownGracePeriod metav1.Duration `json:"shutdownGracePeriod,omitempty"`
	// Etcd contains the etcd configuration.
	Etcd EtcdConfig `json:"etcd"`
	// PlaybookPath is the path for storing temporary playbook files.
	PlaybookPath string `json:"playbookPath"`
}

type TLS struct {
	// CertFile is the file containing x509 Certificate for HTTPS.
	// +optional
	CertFile string `json:"certFile,omitempty"`
	// PrivateKeyFile is the file containing x509 private key matching tlsCertFile
	// +optional
	PrivateKeyFile string `json:"privateKeyFile,omitempty"`
	// CertData contains PEM-encoded data for TLS certificate.
	// +optional
	CertData []byte `json:"certData,omitempty"`
	// PrivateKeyData contains PEM-encoded data for TLS private key.
	// +optional
	PrivateKeyData []byte `json:"privateKeyData,omitempty"`
	// CipherSuites is the list of allowed cipher suites for the server.
	// +optional
	CipherSuites []string `json:"cipherSuites,omitempty"`
	// TLSMinVersion is the minimum TLS version supported.
	// +optional
	TLSMinVersion string `json:"tlsMinVersion,omitempty"`
}

type AgentAuthentication struct {
	// X509 contains settings related to x509 client certificate authentication
	// +optional
	X509 AgentX509Authentication `json:"x509"`
}

type AgentX509Authentication struct {
	// ClientCAFile is the path to a PEM-encoded certificate bundle. If set, any request presenting a client certificate
	// signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	// and groups corresponding to the Organization in the client certificate.
	// +optional
	ClientCAFile string `json:"clientCAFile,omitempty"`
	// ClientCAFile contains PEM-encoded certificate bundle. If set, any request presenting a client certificate
	// signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	// and groups corresponding to the Organization in the client certificate.
	// +optional
	ClientCAData []byte `json:"clientCAData,omitempty"`
}

// EtcdConfig defines etcd configuration
type EtcdConfig struct {
	// DataDir contains the path to the directory for storing etcd data.
	DataDir string `json:"dataDir"`
	// CertsDir contains the path to directory for storing TLS certificates
	CertsDir string `json:"certsDir"`
	// ListenPeerURLs is the list of URLs to listen on for peer traffic.
	ListenPeerURLs string `json:"listenPeerURLs"`
	// ListenClientURLs is the list of URLs to listen on for client traffic
	ListenClientURLs string `json:"listenClientURLs"`
}
