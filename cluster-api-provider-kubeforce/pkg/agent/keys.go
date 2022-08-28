package agent

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/secret"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Keys struct {
	authCA        *certs.KeyPair
	authClient    *certs.KeyPair
	caTLS         []byte
	certTLS       []byte
	privateKeyTLS []byte
}

func GetKeys(ctx context.Context, ctrlclient client.Client, kfAgent *infrav1.KubeforceAgent) (*Keys, error) {
	if kfAgent.Spec.Config.Authentication.X509.CaSecret == "" {
		return nil, errors.New("caSecret is not defined")
	}
	caObjectKey := client.ObjectKey{
		Name:      kfAgent.Spec.Config.Authentication.X509.CaSecret,
		Namespace: kfAgent.Namespace,
	}
	caKeyPair, err := secret.LookupKeyPair(ctx, ctrlclient, caObjectKey)
	if err != nil {
		return nil, err
	}
	if caKeyPair == nil {
		return nil, errors.New("unable to find agent ca")
	}

	if kfAgent.Spec.Config.Authentication.X509.ClientSecret == "" {
		return nil, errors.New("clientSecret is not defined")
	}
	clientObjectKey := client.ObjectKey{
		Name:      kfAgent.Spec.Config.Authentication.X509.ClientSecret,
		Namespace: kfAgent.Namespace,
	}
	clientKeyPair, err := secret.LookupKeyPair(ctx, ctrlclient, clientObjectKey)
	if err != nil {
		return nil, err
	}
	if clientKeyPair == nil {
		return nil, errors.New("unable to find agent client keys")
	}
	if clientKeyPair.Key == nil {
		return nil, errors.New("unable to find agent client private key")
	}

	tlsObjectKey := GetAgentTLSObjectKey(kfAgent)
	s := &corev1.Secret{}
	if err := ctrlclient.Get(ctx, tlsObjectKey, s); err != nil {
		return nil, errors.Wrapf(err, "unable to get agent tls certificate %v", tlsObjectKey)
	}
	return &Keys{
		authCA:        caKeyPair,
		authClient:    clientKeyPair,
		caTLS:         s.Data[secret.TLSCAKey],
		certTLS:       s.Data[corev1.TLSCertKey],
		privateKeyTLS: s.Data[corev1.TLSPrivateKeyKey],
	}, nil
}

func GetAgentTLSObjectKey(kfAgent *v1beta1.KubeforceAgent) client.ObjectKey {
	return client.ObjectKey{
		Name:      fmt.Sprintf("%s-tls", kfAgent.Name),
		Namespace: kfAgent.Namespace,
	}
}
