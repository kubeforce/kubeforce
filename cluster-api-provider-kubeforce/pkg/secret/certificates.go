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

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Certificates are the certificates necessary to bootstrap a cluster.
type Certificates struct {
	agentAuthCA *Certificate
	agentClient *Certificate
}

// NewCertificates creates a new Certificates
func NewCertificates() Certificates {
	return Certificates{
		agentAuthCA: &Certificate{
			Purpose: AgentAuthCA,
		},
		agentClient: &Certificate{
			Purpose: AgentClient,
		},
	}
}

func (c *Certificates) certs() []*Certificate {
	return []*Certificate{c.agentAuthCA, c.agentClient}
}

// Lookup looks up each certificate from secrets and populates the certificate with the secret data.
func (c *Certificates) Lookup(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey) error {
	// Look up each certificate as a secret and populate the certificate/key
	for _, cert := range c.certs() {
		err := cert.Lookup(ctx, ctrlclient, clusterName)
		if err != nil {
			return err
		}
	}
	return nil
}

// Generate will generate any certificates that do not have KeyPair data.
func (c *Certificates) Generate(clusterName string) error {
	if err := c.agentAuthCA.GenerateCA(fmt.Sprintf("agent-ca@%s", clusterName)); err != nil {
		return err
	}
	if err := c.agentClient.GenerateClientCert(fmt.Sprintf("agent-admin@%s", clusterName), c.agentAuthCA.KeyPair); err != nil {
		return err
	}
	return nil
}

// SaveGenerated will save any certificates that have been generated as Kubernetes secrets.
func (c *Certificates) SaveGenerated(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey, owner metav1.OwnerReference) error {
	for _, certificate := range c.certs() {
		if !certificate.Generated {
			continue
		}
		s := certificate.AsSecret(clusterName, owner)
		if err := ctrlclient.Create(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// LookupOrGenerate is a convenience function that wraps cluster bootstrap certificate behavior.
func (c *Certificates) LookupOrGenerate(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey, owner metav1.OwnerReference) error {
	// Find the certificates that exist
	if err := c.Lookup(ctx, ctrlclient, clusterName); err != nil {
		return err
	}

	// Generate the certificates that don't exist
	if err := c.Generate(clusterName.Name); err != nil {
		return err
	}

	// Save any certificates that have been generated
	return c.SaveGenerated(ctx, ctrlclient, clusterName, owner)
}

// Certificate represents a single certificate CA.
type Certificate struct {
	Generated bool
	Purpose   Purpose
	KeyPair   *certs.KeyPair
}

// Lookup looks up each certificate from secrets and populates the certificate with the secret data.
func (c *Certificate) Lookup(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey) error {
	key := client.ObjectKey{
		Name:      Name(clusterName.Name, c.Purpose),
		Namespace: clusterName.Namespace,
	}
	kp, err := LookupKeyPair(ctx, ctrlclient, key)
	if err != nil {
		return err
	}
	c.KeyPair = kp
	return nil
}

func LookupKeyPair(ctx context.Context, ctrlclient client.Client, key client.ObjectKey) (*certs.KeyPair, error) {
	// Look up each certificate as a secret and populate the certificate/key
	s := &corev1.Secret{}
	if err := ctrlclient.Get(ctx, key, s); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	// If a user has a badly formatted secret it will prevent the cluster from working.
	return secretToKeyPair(s)
}

// Hashes hashes all the certificates stored in a CA certificate.
func (c *Certificate) Hashes() ([]string, error) {
	certificates, err := cert.ParseCertsPEM(c.KeyPair.Cert)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse %s certificate", c.Purpose)
	}
	out := make([]string, 0)
	for _, c := range certificates {
		out = append(out, hashCert(c))
	}
	return out, nil
}

// hashCert calculates the sha256 of certificate.
func hashCert(certificate *x509.Certificate) string {
	spkiHash := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
	return "sha256:" + strings.ToLower(hex.EncodeToString(spkiHash[:]))
}

// AsSecret converts a single certificate into a Kubernetes secret.
func (c *Certificate) AsSecret(clusterName client.ObjectKey, owner metav1.OwnerReference) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterName.Namespace,
			Name:      Name(clusterName.Name, c.Purpose),
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName.Name,
			},
		},
		Data: map[string][]byte{
			TLSKeyDataName: c.KeyPair.Key,
			TLSCrtDataName: c.KeyPair.Cert,
		},
		Type: clusterv1.ClusterSecretType,
	}

	if c.Generated {
		s.OwnerReferences = []metav1.OwnerReference{owner}
	}
	return s
}

// GenerateCA generates a certificate.
func (c *Certificate) GenerateCA(commonName string) error {
	if !c.needGenerateCA() {
		return nil
	}
	kp, err := generateCACert(commonName)
	if err != nil {
		return err
	}
	c.KeyPair = kp
	c.Generated = true

	return nil
}

func (c *Certificate) GenerateClientCert(commonName string, ca *certs.KeyPair) error {
	needGenerate, err := c.needGenerateClient(certs.ClientCertificateRenewalDuration)
	if err != nil {
		return err
	}
	if !needGenerate {
		return nil
	}
	kp, err := generateClientKeys(commonName, ca)
	if err != nil {
		return err
	}
	c.KeyPair = kp
	c.Generated = true

	return nil
}

func (c *Certificate) needGenerateCA() bool {
	return c.KeyPair == nil
}

func (c *Certificate) needGenerateClient(threshold time.Duration) (bool, error) {
	if c.KeyPair == nil {
		return true, nil
	}
	crt, err := certs.DecodeCertPEM(c.KeyPair.Cert)
	if err != nil {
		return false, err
	}
	now := time.Now()
	if crt.NotAfter.Sub(now) < threshold {
		return true, nil
	}
	return false, nil
}

func secretToKeyPair(s *corev1.Secret) (*certs.KeyPair, error) {
	c, exists := s.Data[TLSCrtDataName]
	if !exists {
		return nil, errors.Errorf("missing data for key %s", TLSCrtDataName)
	}

	key, exists := s.Data[TLSKeyDataName]
	if !exists {
		key = []byte("")
	}

	return &certs.KeyPair{
		Cert: c,
		Key:  key,
	}, nil
}

func generateCACert(commonName string) (*certs.KeyPair, error) {
	x509Cert, privKey, err := newCertificateAuthority(commonName)
	if err != nil {
		return nil, err
	}
	return &certs.KeyPair{
		Cert: certs.EncodeCertPEM(x509Cert),
		Key:  certs.EncodePrivateKeyPEM(privKey),
	}, nil
}

func generateClientKeys(commonName string, ca *certs.KeyPair) (*certs.KeyPair, error) {
	cfg := &certs.Config{
		CommonName:   commonName,
		Organization: []string{"system:masters"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientKey, err := certs.NewPrivateKey()
	if err != nil {
		return nil, errors.Wrap(err, "unable to create private key")
	}
	caKey, err := certs.DecodePrivateKeyPEM(ca.Key)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse CA private key")
	}
	caCert, err := certs.DecodeCertPEM(ca.Cert)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse CA public cert")
	}
	clientCert, err := cfg.NewSignedCert(clientKey, caCert, caKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to sign certificate")
	}

	return &certs.KeyPair{
		Cert: certs.EncodeCertPEM(clientCert),
		Key:  certs.EncodePrivateKeyPEM(clientKey),
	}, nil
}

// newCertificateAuthority creates new certificate and private key for the certificate authority.
func newCertificateAuthority(commonName string) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := certs.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	c, err := newSelfSignedCACert(commonName, key)
	if err != nil {
		return nil, nil, err
	}

	return c, key, nil
}

// newSelfSignedCACert creates a CA certificate.
func newSelfSignedCACert(commonName string, key *rsa.PrivateKey) (*x509.Certificate, error) {
	cfg := certs.Config{
		CommonName: commonName,
	}
	now := time.Now().UTC()

	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		NotBefore:             now.Add(time.Minute * -5),
		NotAfter:              now.Add(time.Hour * 24 * 365 * 10), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		MaxPathLenZero:        true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		IsCA:                  true,
	}

	b, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create self signed CA certificate: %+v", tmpl)
	}

	c, err := x509.ParseCertificate(b)
	return c, errors.WithStack(err)
}
