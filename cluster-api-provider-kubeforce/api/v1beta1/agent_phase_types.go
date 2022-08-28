package v1beta1

// AgentPhase is a string representation of a KubeforceAgent Phase.
//
// Controllers should always look at the actual state of the KubeforceAgent’s fields to make those decisions.
type AgentPhase string

const (
	// AgentPhasePending is the first state an Agent is assigned by
	AgentPhasePending AgentPhase = "Pending"

	// AgentPhaseProvisioning is the state when the
	// Agent infrastructure is being created.
	AgentPhaseProvisioning AgentPhase = "Provisioning"

	// AgentPhaseInstalled is the state when agent has been installed and configured.
	AgentPhaseInstalled AgentPhase = "Installed"

	// AgentPhaseRunning is the Agent state when it has
	// become a Kubernetes Node in a Ready state.
	AgentPhaseRunning AgentPhase = "Running"

	// AgentPhaseDeleting is the Agent state when a delete
	// request has been sent to the API Server,
	// but its infrastructure has not yet been fully deleted.
	AgentPhaseDeleting AgentPhase = "Deleting"

	// AgentPhaseFailed is the Agent state when the system
	// might require user intervention.
	AgentPhaseFailed AgentPhase = "Failed"

	// AgentPhaseUnknown is returned if the Agent state cannot be determined.
	AgentPhaseUnknown AgentPhase = "Unknown"
)
