/*
Copyright 2021 The Kubeforce Authors.

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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Conditions and condition Reasons for the KubeforceAgent object.
const (
	// AgentInstalledCondition documents the status of the installing of the agent.
	AgentInstalledCondition clusterv1.ConditionType = "AgentInstalled"

	// WaitingForExternalAddressesReason (Severity=Info).
	WaitingForExternalAddressesReason = "WaitingForExternalAddresses"

	// WaitingForSSHConfigurationReason (Severity=Info).
	WaitingForSSHConfigurationReason = "WaitingForSSHConfiguration"

	// WaitingForAgentToRunReason (Severity=Info) documents a KubeforceAgent waiting for the agent
	// to run that provides the KubeforceMachine infrastructure.
	WaitingForAgentToRunReason = "WaitingForAgentToRun"

	// AgentInstallingFailedReason (Severity=Error) documents a KubeforceAgent controller detecting
	// an error while installing the agent; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	AgentInstallingFailedReason = "AgentInstallingFailed"
)

const (
	// HealthyCondition documents the health state of an agent.
	HealthyCondition clusterv1.ConditionType = "Healthy"
	// ProbeFailedReason (Severity=Error) documents a KubeforceAgent that controller can not connect to the agent.
	ProbeFailedReason = "ProbeFailed"
)

const (
	// AgentTLSCondition documents the status of the agent tls certificate.
	AgentTLSCondition clusterv1.ConditionType = "AgentTLS"

	// WaitingForCertIssuerRefReason (Severity=Error) documents a KubeforceAgent waiting for the certificate
	// issuer reference to be installed.
	WaitingForCertIssuerRefReason = "WaitingForCertIssuerRef"

	// WaitingForCertIssueReason (Severity=Info) documents a KubeforceAgent waiting for the issue of the tls certificate
	// for the agent.
	WaitingForCertIssueReason = "WaitingForCertIssue"
)

const (
	// AgentInfoCondition documents the receipt of version and system information from an agent.
	AgentInfoCondition clusterv1.ConditionType = "AgentInfo"
	// AgentInfoWaitingForConnectionReadiness (Severity=Info) documents a KubeforceAgent waiting to be ready to connect.
	AgentInfoWaitingForConnectionReadiness = "WaitingForConnectionReadiness"
	// AgentInfoFailedReason (Severity=Error) documents that controller can not get the agent information.
	AgentInfoFailedReason = "AgentInfoFailedReason"
)

const (
	// InitPlaybooksCondition provides an observation for the Playbooks during initialization process of the Object.
	InitPlaybooksCondition clusterv1.ConditionType = "InitPlaybooksCompleted"

	// WaitingForBootstrapDataReason (Severity=Info) documents a KubeforceMachine waiting for the bootstrap
	// script to be ready before starting to create the container that provides the KubeforceMachine infrastructure.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"

	// PlaybooksDeployingFailedReason (Severity=Error) documents a KubeforceMachine detecting
	// an error while deploying playbooks.
	PlaybooksDeployingFailedReason = "PlaybooksDeployingFailed"

	// WaitingForCompletionPhaseReason (Severity=Info).
	WaitingForCompletionPhaseReason = "WaitingForCompletionPhase"

	// WaitingForClusterInfrastructureReason (Severity=Info) documents a KubeforceMachine waiting for the cluster
	// infrastructure to be ready before starting.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
)

const (
	// CleanupPlaybooksCondition provides an observation for the Playbooks of the cleanup process of the Object.
	CleanupPlaybooksCondition clusterv1.ConditionType = "CleanupPlaybooksCompleted"
)

const (
	// InfrastructureAvailableCondition documents the deployment status of the playbooks for KubeforceMachine.
	InfrastructureAvailableCondition clusterv1.ConditionType = "InfrastructureAvailable"
)

const (
	// CertificatesAvailableCondition documents that cluster certificates are available.
	CertificatesAvailableCondition clusterv1.ConditionType = "CertificatesAvailable"

	// CertificatesGenerationFailedReason (Severity=Warning) documents a KubeforceCluster controller detecting
	// an error while generating certificates; those kind of errors are usually temporary and the controller
	// automatically recover from them.
	CertificatesGenerationFailedReason = "CertificatesGenerationFailed"

	// CertificatesCorruptedReason (Severity=Error) documents a KubeforceCluster controller detecting
	// an error while retrieving certificates for a joining node.
	CertificatesCorruptedReason = "CertificatesCorrupted"
)

// Conditions and condition Reasons for the KubeforceMachine object.
const (
	// AgentProvisionedCondition documents the status of the provisioning of the agent
	// generated by a KubeforceMachine.
	//
	AgentProvisionedCondition clusterv1.ConditionType = "AgentProvisioned"

	// WaitingForAgentReason (Severity=Info) documents a KubeforceMachine waiting for the
	// agent to be ready.
	WaitingForAgentReason = "WaitingForAgent"

	// AgentProvisioningFailedReason (Severity=Warning) documents a KubeforceMachine controller detecting
	// an error while provisioning the container that provides the KubeforceMachine infrastructure; those kind of
	// errors are usually transient and failed provisioning are automatically re-tried by the controller.
	AgentProvisioningFailedReason = "AgentProvisioningFailed"

	// AgentDeletedReason (Severity=Error) documents a KubeforceMachine controller detecting
	// the underlying container has been deleted unexpectedly.
	AgentDeletedReason = "AgentDeleted"
)

// Conditions and condition Reasons for the object.
const (
	// SynchronizationCondition documents the status of synchronization with a remote object.
	SynchronizationCondition clusterv1.ConditionType = "Synced"

	// WaitingForObservedGenerationReason (Severity=Info).
	WaitingForObservedGenerationReason = "WaitingForObservedGeneration"

	// SynchronizationFailedReason (Severity=Error) documents a controller detecting
	// an error while synchronizing the object.
	SynchronizationFailedReason = "SynchronizationFailed"
)
const (
	// BootstrapExecSucceededCondition provides an observation of the KubeforceMachine bootstrap process.
	BootstrapExecSucceededCondition clusterv1.ConditionType = "BootstrapExecSucceeded"

	// BootstrappingReason documents (Severity=Info) a KubeforceMachine currently executing the bootstrap
	// script that creates the Kubernetes node on the newly provisioned machine infrastructure.
	BootstrappingReason = "Bootstrapping"

	// BootstrapFailedReason documents (Severity=Warning) a KubeforceMachine controller detecting an error while
	// bootstrapping the Kubernetes node on the machine just provisioned.
	BootstrapFailedReason = "BootstrapFailed"
)

const (
	// ProviderIDSucceededCondition monitors the provision of the ProviderID.
	ProviderIDSucceededCondition clusterv1.ConditionType = "ProviderIDProvisioned"

	// ProviderIDFailedReason documents (Severity=Error) a KubeforceMachine controller detecting an error while
	// provision ProviderID.
	ProviderIDFailedReason = "ProviderIDFailed"
)

// Conditions and condition Reasons for the KubeforceCluster object.

const (
	// LoadBalancerAvailableCondition documents the availability of the internal load balancer for cluster.
	LoadBalancerAvailableCondition clusterv1.ConditionType = "LoadBalancerAvailable"

	// LoadBalancerProvisioningFailedReason (Severity=Warning) documents a KubeforceCluster controller detecting
	// an error while provisioning internal load balancer.; those kind of
	// errors are usually transient and failed provisioning are automatically re-tried by the controller.
	LoadBalancerProvisioningFailedReason = "LoadBalancerProvisioningFailed"
)
