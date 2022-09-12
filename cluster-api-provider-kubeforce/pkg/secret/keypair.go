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

package secret

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KeyPair holds the raw bytes for a certificate and key.
type KeyPair struct {
	CA, Cert, Key []byte
}

func LookupKeyPair(ctx context.Context, ctrlclient client.Client, key client.ObjectKey) (*KeyPair, error) {
	// Look up each certificate as a secret and populate the certificate/key
	s := &corev1.Secret{}
	if err := ctrlclient.Get(ctx, key, s); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	return secretToKeyPair(s)
}

func secretToKeyPair(s *corev1.Secret) (*KeyPair, error) {
	caCrt, exists := s.Data[TLSCAKey]
	if !exists {
		return nil, errors.Errorf("missing data for key %s in the secret %q", TLSCAKey, client.ObjectKeyFromObject(s))
	}

	crt, exists := s.Data[TLSCertKey]
	if !exists {
		return nil, errors.Errorf("missing data for key %s in the secret %q", TLSCertKey, client.ObjectKeyFromObject(s))
	}

	key, exists := s.Data[TLSPrivateKeyKey]
	if !exists {
		return nil, errors.Errorf("missing data for key %s in the secret %q", TLSPrivateKeyKey, client.ObjectKeyFromObject(s))
	}

	return &KeyPair{
		CA:   caCrt,
		Cert: crt,
		Key:  key,
	}, nil
}
