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

package envtest

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k3f.io/kubeforce/agent/pkg/apiserver"
	"k3f.io/kubeforce/agent/pkg/config"
	"k3f.io/kubeforce/agent/pkg/manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Environment struct {
	tmpDir string
	config *config.Config
}

func (e *Environment) generateConfig() error {
	tmpDir, err := ioutil.TempDir("", "kubeforce-agent-")
	if err != nil {
		return err
	}
	e.tmpDir = tmpDir
	ipAddresses := []net.IP{
		net.IPv4(127, 0, 0, 1),
	}
	certPem, keyPem, err := certutil.GenerateSelfSignedCertKey("kubeforce-agent", ipAddresses, nil)
	if err != nil {
		return err
	}
	cfg := &config.Config{
		Spec: config.ConfigSpec{
			Port: 15443,
			TLS: config.TLS{
				CertData:       certPem,
				PrivateKeyData: keyPem,
				TLSMinVersion:  "VersionTLS13",
			},
			Authentication: config.AgentAuthentication{
				X509: config.AgentX509Authentication{
					ClientCAData: certPem,
				},
			},
			ShutdownGracePeriod: metav1.Duration{
				Duration: 30 * time.Second,
			},
			Etcd: config.EtcdConfig{
				DataDir:          filepath.Join(tmpDir, "etcd-data"),
				CertsDir:         filepath.Join(tmpDir, "etcd-certs"),
				ListenPeerURLs:   "https://127.0.0.1:12380",
				ListenClientURLs: "https://127.0.0.1:12379",
			},
			PlaybookPath: filepath.Join(tmpDir, "playbook"),
		},
	}
	e.config = cfg
	return nil
}

func (e *Environment) Start(ctx context.Context, runnable func(agentConfig *config.Config, config *rest.Config) manager.RunnableFunc) (*rest.Config, error) {
	if err := e.generateConfig(); err != nil {
		return nil, err
	}

	log := ctrlzap.NewRaw(ctrlzap.UseDevMode(true), ctrlzap.Level(zapcore.DebugLevel))
	zap.ReplaceGlobals(log)
	logger := zapr.NewLogger(log)
	ctrl.SetLogger(logger)
	scheme := apiserver.Scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	mgr, err := manager.NewManager(e.config.Spec.ShutdownGracePeriod.Duration)
	if err != nil {
		return nil, err
	}
	etcdSrv, err := apiserver.NewEtcdServer(e.config.Spec.Etcd)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create etcd server")
	}
	mgr.Add(etcdSrv)
	srv, err := apiserver.NewServer(e.config.Spec)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create apiserver")
	}
	mgr.Add(srv)
	mgr.Add(runnable(e.config, srv.LoopbackClientConfig))
	go func() {
		if err := mgr.Start(ctx); err != nil {
			logger.Error(err, "problem running manager")
		}
		_ = os.RemoveAll(e.tmpDir)
	}()

	return srv.LoopbackClientConfig, nil
}
