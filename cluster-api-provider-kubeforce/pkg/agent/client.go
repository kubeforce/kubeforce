package agent

import (
	"net"
	"time"

	"github.com/pkg/errors"
	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/util/conditions"
)

func NewClientConfigByAddress(keys *Keys, addresses infrav1.Addresses) (*restclient.Config, error) {
	host, err := GetServer(addresses)
	if err != nil {
		return nil, err
	}
	return NewClientConfig(keys, host), nil
}

func GetServer(addresses infrav1.Addresses) (string, error) {
	address := stringutil.Find(stringutil.IsNotEmpty, addresses.ExternalDNS, addresses.ExternalIP)
	if address == "" {
		return "", errors.Errorf("not found external address")
	}
	server := "https://" + net.JoinHostPort(address, "5443")
	return server, nil
}

func NewClientConfig(keys *Keys, host string) *restclient.Config {
	config := restclient.Config{
		QPS:     restclient.DefaultQPS,
		Burst:   restclient.DefaultBurst,
		Timeout: 10 * time.Second,
		Host:    host,
		TLSClientConfig: restclient.TLSClientConfig{
			CertData:   keys.authClient.Cert,
			KeyData:    keys.authClient.Key,
			CAData:     keys.tls.CA,
			NextProtos: []string{"h2"},
		},
	}
	if len(config.TLSClientConfig.CAData) == 0 {
		config.TLSClientConfig.CAData = keys.tls.Cert
	}
	return &config
}

func NewClientKubeconfig(keys *Keys, server string) api.Config {
	clusterName := "kubernetes"
	contextName := "default"
	username := "agent"
	clusters := make(map[string]*api.Cluster)
	clusters[clusterName] = &api.Cluster{
		Server:                   server,
		CertificateAuthorityData: keys.tls.CA,
	}

	contexts := make(map[string]*api.Context)
	contexts[contextName] = &api.Context{
		Cluster:  clusterName,
		AuthInfo: username,
	}

	authinfos := make(map[string]*api.AuthInfo)
	authinfos[username] = &api.AuthInfo{
		ClientCertificateData: keys.authClient.Cert,
		ClientKeyData:         keys.authClient.Key,
	}

	apiConfig := api.Config{
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: contextName,
		AuthInfos:      authinfos,
	}
	return apiConfig
}

func NewClientSet(keys *Keys, addresses infrav1.Addresses) (*clientset.Clientset, error) {
	config, err := NewClientConfigByAddress(keys, addresses)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(config)
}

func IsReady(a *infrav1.KubeforceAgent) bool {
	return a.Spec.Installed && conditions.IsTrue(a, infrav1.Healthy)
}
