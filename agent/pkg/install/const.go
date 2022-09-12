package install

const (
	serviceName = "kubeforce-agent.service"
	agentPath   = "/usr/local/bin/kubeforce-agent"
	configPath  = "/var/lib/kubeforce/config.yaml"
	servicePath = "/etc/systemd/system/" + serviceName

	certsDir       = "/etc/kubeforce/certs/"
	certFile       = certsDir + "tls.crt"
	privateKeyFile = certsDir + "tls.key"
	clientCAFile   = certsDir + "client-ca.crt"
)
