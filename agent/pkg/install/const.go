package install

const (
	serviceName = "kubeforce-agent.service"
	agentPath   = "/usr/local/bin/kubeforce-agent"
	configPath  = "/var/lib/kubeforce/config.yaml"
	servicePath = "/etc/systemd/system/" + serviceName
)
