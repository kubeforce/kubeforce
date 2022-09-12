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

package agent

import (
	"context"
	"fmt"

	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/names"

	"github.com/pkg/errors"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Keys struct {
	authClient *secret.KeyPair
	tls        *secret.KeyPair
}

func GetKeys(ctx context.Context, ctrlclient client.Client, kfAgent *infrav1.KubeforceAgent) (*Keys, error) {
	clientObjectKey, err := GetAgentClientCertObjectKey(kfAgent, ActiveKey)
	if err != nil {
		return nil, err
	}
	clientKeyPair, err := secret.LookupKeyPair(ctx, ctrlclient, *clientObjectKey)
	if err != nil {
		return nil, err
	}
	if clientKeyPair == nil {
		return nil, errors.New("unable to find agent client cert")
	}
	tlsObjectKey := GetAgentTLSObjectKey(kfAgent, ActiveKey)
	tlsclientKeyPair, err := secret.LookupKeyPair(ctx, ctrlclient, tlsObjectKey)
	if err != nil {
		return nil, err
	}
	if tlsclientKeyPair == nil {
		return nil, errors.New("unable to find agent tls cert")
	}
	return &Keys{
		authClient: clientKeyPair,
		tls:        tlsclientKeyPair,
	}, nil
}

// PurposeKey specifies the purpose of the tls agent key
type PurposeKey string

const (
	IssuedKey PurposeKey = "tls"
	ActiveKey PurposeKey = "tls-active"
)

func GetAgentTLSObjectKey(kfAgent *v1beta1.KubeforceAgent, suffix PurposeKey) client.ObjectKey {
	return client.ObjectKey{

		Name:      names.BuildName(kfAgent.Name, "-"+string(suffix)),
		Namespace: kfAgent.Namespace,
	}
}

func GetAgentClientCertObjectKey(kfAgent *v1beta1.KubeforceAgent, key PurposeKey) (*client.ObjectKey, error) {
	if kfAgent.Spec.Config.Authentication.X509.ClientSecret == "" {
		return nil, errors.New("clientSecret is not defined")
	}
	switch key {
	case IssuedKey:
		return &client.ObjectKey{
			Name:      kfAgent.Spec.Config.Authentication.X509.ClientSecret,
			Namespace: kfAgent.Namespace,
		}, nil
	case ActiveKey:
		suffix := fmt.Sprintf("-%s-active", kfAgent.Name)
		return &client.ObjectKey{
			Name:      names.BuildName(kfAgent.Spec.Config.Authentication.X509.ClientSecret, suffix),
			Namespace: kfAgent.Namespace,
		}, nil
	default:
		return nil, errors.Errorf("unknown key %s", key)
	}
}
