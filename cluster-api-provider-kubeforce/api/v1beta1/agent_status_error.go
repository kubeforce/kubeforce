package v1beta1

// AgentStatusError defines errors states for KubeforceAgent objects.
type AgentStatusError string

const (
	// InvalidConfigurationAgentError represents that the combination
	// of configuration in the AgentSpec is not supported.
	// This is not a transient error, but
	// indicates a state that must be fixed before progress can be made.
	InvalidConfigurationAgentError AgentStatusError = "InvalidConfiguration"

	// InstallAgentError indicates an error while trying to install a Agent to the node.
	// This may indicate a transient issue that will be fixed automatically resolved with time,
	// such as a node failure or the bastion host is unavailable.
	InstallAgentError AgentStatusError = "InstallError"

	// UpdateAgentError indicates an error while trying to update a Agent that this
	// Agent represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage,
	//
	// Example: error updating load balancers.
	UpdateAgentError AgentStatusError = "UpdateError"

	// DeleteAgentError indicates an error was encountered while trying to delete the Node that this
	// Agent represents. This could be a transient or terminal error, but
	// will only be observable if the Agent controller has
	// added a finalizer to the object to more gracefully handle deletions.
	DeleteAgentError AgentStatusError = "DeleteError"
)
