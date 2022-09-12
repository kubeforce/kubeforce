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

package apiserver

import (
	"bytes"
	"context"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/util/keyutil"

	certutil "k8s.io/client-go/util/cert"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/embed"
	"k3f.io/kubeforce/agent/pkg/config"
)

const duration365d = time.Hour * 24 * 365
const etcdCaBaseName = "etcd-ca"
const etcdCertBaseName = "etcd-server"
const etcdClientBaseName = "etcd-client"

// NewEtcdServer creates embedded etcd server
func NewEtcdServer(cfg config.EtcdConfig) (*EtcdServer, error) {
	err := generateCerts(cfg.CertsDir)
	if err != nil {
		return nil, err
	}
	etcdCfg := embed.NewConfig()
	etcdCfg.Dir = cfg.DataDir
	if cfg.ListenPeerURLs != "" {
		u, err := types.NewURLs(strings.Split(cfg.ListenPeerURLs, ","))
		if err != nil {
			return nil, errors.Wrapf(err, "unexpected error setting up listenPeerURLs: %q", cfg.ListenPeerURLs)
		}
		etcdCfg.LPUrls = u
	}

	if cfg.ListenClientURLs != "" {
		u, err := types.NewURLs(strings.Split(cfg.ListenClientURLs, ","))
		if err != nil {
			return nil, errors.Wrapf(err, "unexpected error setting up listenClientURLs: %q", cfg.ListenClientURLs)
		}
		etcdCfg.LCUrls = u
	}
	etcdCfg.Logger = "zap"
	tslInfo := transport.TLSInfo{
		CertFile:       certFilePath(cfg.CertsDir, etcdCertBaseName),
		KeyFile:        keyFilePath(cfg.CertsDir, etcdCertBaseName),
		TrustedCAFile:  certFilePath(cfg.CertsDir, etcdCaBaseName),
		ClientCertAuth: true,
		ClientCertFile: certFilePath(cfg.CertsDir, etcdClientBaseName),
		ClientKeyFile:  keyFilePath(cfg.CertsDir, etcdClientBaseName),
	}
	etcdCfg.ClientTLSInfo = tslInfo
	etcdCfg.PeerTLSInfo = tslInfo
	s := &EtcdServer{
		cfg:     etcdCfg,
		started: make(chan struct{}),
	}
	return s, nil
}

func certFilePath(dir, baseName string) string {
	return filepath.Join(dir, baseName+".crt")
}

func keyFilePath(dir, baseName string) string {
	return filepath.Join(dir, baseName+".key")
}

func generateCerts(certsDir string) error {
	err := os.MkdirAll(certsDir, 0755)
	if err != nil {
		return err
	}
	caKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return errors.Wrap(err, "unable to generate CA rsa key")
	}
	caCert, err := generateCACert(certsDir, etcdCaBaseName, "Etcd CA", []string{"Kubeforce Inc"}, caKey)
	if err != nil {
		return errors.Wrap(err, "unable to generate CA cert for etcd")
	}
	ipAddresses := []net.IP{
		net.IPv4(127, 0, 0, 1),
	}
	_, err = generateCert(certsDir, etcdCertBaseName, "Etcd Server",
		ipAddresses, nil, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, caCert, caKey)
	if err != nil {
		return errors.Wrap(err, "unable to generate server cert for etcd")
	}
	_, err = generateCert(certsDir, etcdClientBaseName, "Etcd Client",
		ipAddresses, nil, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, caCert, caKey)
	if err != nil {
		return errors.Wrap(err, "unable to generate client cert for etcd")
	}
	return nil
}

type EtcdServer struct {
	cfg     *embed.Config
	started chan struct{}
	Etcd    *embed.Etcd
}

func (s *EtcdServer) Start(ctx context.Context) error {
	e, err := embed.StartEtcd(s.cfg)
	if err != nil {
		return err
	}
	select {
	case <-e.Server.ReadyNotify():
		s.Etcd = e
		close(s.started)
	case <-ctx.Done():
		e.Server.Stop()
		return nil
	}
	<-ctx.Done()
	e.Server.Stop()
	return nil
}

// ReadyNotify returns a channel that will be closed when the server
// is ready to serve client requests
func (s *EtcdServer) ReadyNotify() <-chan struct{} {
	return s.started
}

func generateCACert(dir, baseName, commonName string, org []string, key crypto.Signer) (*x509.Certificate, error) {
	validFrom := time.Now().Add(-time.Hour) // valid an hour earlier to avoid flakes due to clock skew
	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: org,
		},
		DNSNames:              []string{commonName},
		NotBefore:             validFrom.UTC(),
		NotAfter:              validFrom.Add(duration365d * 10).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: certutil.CertificateBlockType, Bytes: certDERBytes}); err != nil {
		return nil, err
	}

	keyBytes, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, err
	}
	certPath := certFilePath(dir, baseName)
	keyPath := keyFilePath(dir, baseName)
	if err := ioutil.WriteFile(certPath, certBuffer.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("failed to write cert to %s: %v", certPath, err)
	}
	if err := ioutil.WriteFile(keyPath, keyBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write key to %s: %v", keyPath, err)
	}
	return cert, err
}

func generateCert(dir, baseName, commonName string, ipAddresses []net.IP,
	dnsNames []string, keyUsages []x509.ExtKeyUsage, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, error) {
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate rsa key")
	}
	validFrom := time.Now().Add(-time.Hour) // valid an hour earlier to avoid flakes due to clock skew
	maxAge := time.Hour * 24 * 365          // one year for certs
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: validFrom,
		NotAfter:  validFrom.Add(maxAge),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           keyUsages,
		BasicConstraintsValid: true,
		IPAddresses:           ipAddresses,
		DNSNames:              dnsNames,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &template, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}
	certPath := certFilePath(dir, baseName)
	keyPath := keyFilePath(dir, baseName)
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: certutil.CertificateBlockType, Bytes: certDERBytes}); err != nil {
		return nil, err
	}

	keyBytes, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(certPath, certBuffer.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("failed to write cert fixture to %s: %v", certPath, err)
	}
	if err := ioutil.WriteFile(keyPath, keyBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write key fixture to %s: %v", keyPath, err)
	}
	return cert, nil
}
