/*
Copyright 2019 The Kubernetes Authors.

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

package secret

// Purpose is the name to append to the secret generated for a cluster.
type Purpose string

const (
	// TLSCAKey is the key used to store a CA certificate in the secret's data field.
	TLSCAKey = "ca.crt"

	// TLSKeyDataName is the key used to store a TLS private key in the secret's data field.
	TLSKeyDataName = "tls.key"

	// TLSCrtDataName is the key used to store a TLS certificate in the secret's data field.
	TLSCrtDataName = "tls.crt"

	// SSHAuthPassword is the password of the SSH configuration
	SSHAuthPassword = "ssh-password"

	// SSHAuthPassphrase is the passphrase of the SSH private key
	SSHAuthPassphrase = "ssh-passphrase"

	// AgentAuthCA is the secret name suffix for Agent CA.
	AgentAuthCA = Purpose("agent-auth-ca")

	// AgentClient is the secret name suffix for Agent Client certificate.
	AgentClient = Purpose("agent-client")
)
