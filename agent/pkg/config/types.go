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

package config

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Config defines agent configuration.
type Config struct {
	metav1.TypeMeta
	Spec ConfigSpec
}

// ConfigSpec defines agent configuration.
//
//nolint:revive
type ConfigSpec struct {
	// Port is the port for the Agent to serve on.
	Port int
	// TLS specifies tls configuration for the server.
	TLS TLS
	// authentication specifies how requests to the Agent's server are authenticated
	Authentication AgentAuthentication
	// ShutdownGracePeriod specifies the total grace period  for shutdown the server.
	ShutdownGracePeriod metav1.Duration
	// Etcd contains the etcd configuration.
	Etcd EtcdConfig
	// PlaybookPath is the path for storing temporary playbook files.
	PlaybookPath string
}

// TLS describes the tls certificate.
type TLS struct {
	// CertFile is the file containing x509 Certificate for HTTPS.
	// +optional
	CertFile string
	// CertData contains PEM-encoded data for TLS certificate.
	// +optional
	CertData []byte
	// PrivateKeyFile is the file containing x509 private key matching tlsCertFile
	// +optional
	PrivateKeyFile string
	// PrivateKeyData contains PEM-encoded data for TLS private key.
	// +optional
	PrivateKeyData []byte
	// CipherSuites is the list of allowed cipher suites for the server.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	// +optional
	CipherSuites []string
	// TLSMinVersion is the minimum TLS version supported.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	// +optional
	TLSMinVersion string
}

// AgentAuthentication is configuration for agent authentication.
type AgentAuthentication struct {
	// X509 contains settings related to x509 client certificate authentication
	// +optional
	X509 AgentX509Authentication
}

// AgentX509Authentication describes configuration of x509 client certificate authentication.
type AgentX509Authentication struct {
	// ClientCAFile is the path to a PEM-encoded certificate bundle. If set, any request presenting a client certificate
	// signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	// and groups corresponding to the Organization in the client certificate.
	// +optional
	ClientCAFile string
	// ClientCAFile contains PEM-encoded certificate bundle. If set, any request presenting a client certificate
	// signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	// and groups corresponding to the Organization in the client certificate.
	// +optional
	ClientCAData []byte
}

// EtcdConfig defines etcd configuration.
type EtcdConfig struct {
	// DataDir contains the path to the directory for storing etcd data.
	DataDir string
	// CertsDir contains the path to directory for storing TLS certificates
	CertsDir string
	// ListenPeerURLs is the list of URLs to listen on for peer traffic.
	ListenPeerURLs string
	// ListenClientURLs is the list of URLs to listen on for client traffic
	ListenClientURLs string
}

var _ fmt.Stringer = new(TLS)
var _ fmt.GoStringer = new(TLS)

// GoString implements fmt.GoStringer and sanitizes sensitive fields of
// TLS to prevent accidental leaking via logs.
func (c TLS) GoString() string {
	return c.String()
}

// String implements fmt.Stringer and sanitizes sensitive fields of TLS
// to prevent accidental leaking via logs.
func (c TLS) String() string {
	privKeyData := "[]byte(nil)"
	if len(c.PrivateKeyData) > 0 {
		privKeyData = "[]byte{--- REDACTED ---}"
	}
	certData := "[]byte(nil)"
	if len(c.CertData) > 0 {
		certData = "[]byte{--- REDACTED ---}"
	}
	return fmt.Sprintf("config.TLS{CertFile: %q, CertData: %s, PrivateKeyFile: %q, PrivateKeyData: %s, CipherSuites:%v, TLSMinVersion: %q}",
		c.CertFile, certData, c.PrivateKeyFile, privKeyData, c.CipherSuites, c.TLSMinVersion)
}
