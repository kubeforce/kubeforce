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

package install

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"k3f.io/kubeforce/agent/pkg/config"
	configutils "k3f.io/kubeforce/agent/pkg/config/utils"
)

var (
	//go:embed assets/kubeforce-agent.service
	agentServiceContent string
)

// Install installs a agent as the systemd service and runs it.
func Install(ctx context.Context, cfg config.Config) error {
	if err := stopService(ctx); err != nil {
		return err
	}
	if err := copyBinary(); err != nil {
		return err
	}
	if err := copyTLSCerts(cfg.Spec); err != nil {
		return err
	}
	if err := copyClientCACert(cfg.Spec); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	if err := createService(ctx); err != nil {
		return err
	}
	return nil
}

func copyBinary() error {
	exPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(agentPath), 0755); err != nil {
		return err
	}
	if err := copyFile(agentPath, exPath, 0755); err != nil {
		return err
	}
	if err := os.Chmod(agentPath, 0755); err != nil {
		return err
	}
	if err := os.Chown(agentPath, 0, 0); err != nil {
		return err
	}
	return nil
}

func copyTLSCerts(cfg config.ConfigSpec) error {
	if len(cfg.TLS.CertData) != 0 || len(cfg.TLS.PrivateKeyData) != 0 {
		if _, err := saveFile(certFile, cfg.TLS.CertData, 0o600, 0o777); err != nil {
			return err
		}
		if _, err := saveFile(privateKeyFile, cfg.TLS.PrivateKeyData, 0o600, 0o777); err != nil {
			return err
		}
	} else if cfg.TLS.CertFile != "" || cfg.TLS.PrivateKeyFile != "" {
		if err := copyFile(certFile, cfg.TLS.CertFile, 0600); err != nil {
			return err
		}
		if err := copyFile(privateKeyFile, cfg.TLS.PrivateKeyFile, 0600); err != nil {
			return err
		}
	} else {
		return errors.New("tls certificate and private key is not defined")
	}
	return nil
}

func copyClientCACert(cfg config.ConfigSpec) error {
	if len(cfg.Authentication.X509.ClientCAData) > 0 {
		if _, err := saveFile(clientCAFile, cfg.Authentication.X509.ClientCAData, 0o600, 0o777); err != nil {
			return err
		}
	} else if len(cfg.Authentication.X509.ClientCAFile) > 0 {
		if err := copyFile(clientCAFile, cfg.Authentication.X509.ClientCAFile, 0600); err != nil {
			return err
		}
	} else {
		return errors.New("authentication is not configured")
	}
	return nil
}

func saveConfig(cfg config.Config) error {
	cfg = *cfg.DeepCopy()
	cfg.Spec.TLS.CertFile = certFile
	cfg.Spec.TLS.CertData = nil
	cfg.Spec.TLS.PrivateKeyFile = privateKeyFile
	cfg.Spec.TLS.PrivateKeyData = nil
	cfg.Spec.Authentication.X509.ClientCAFile = clientCAFile
	cfg.Spec.Authentication.X509.ClientCAData = nil

	if err := os.MkdirAll(filepath.Dir(configPath), 0o600); err != nil {
		return err
	}
	data, err := configutils.Marshal(&cfg)
	if err != nil {
		return err
	}

	if _, err := saveFile(configPath, data, 0o600, 0o777); err != nil {
		return err
	}
	if err := os.Chmod(configPath, 0o600); err != nil {
		return err
	}
	return nil
}

func stopService(ctx context.Context) error {
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to dbus")
	}
	defer conn.Close()

	unitStatuses, err := conn.ListUnitsByNamesContext(ctx, []string{serviceName})
	if err != nil {
		return errors.Wrap(err, "unable to get a systemd units")
	}
	if len(unitStatuses) == 0 {
		return errors.Errorf("unable to get status for systemd unit %s", serviceName)
	}
	unitStatus := unitStatuses[0]
	needStop := false
	if unitStatus.ActiveState == "active" {
		needStop = true
	}
	if !needStop {
		return nil
	}
	responseCh := make(chan string)
	defer close(responseCh)

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", responseCh); err != nil {
		return errors.Wrapf(err, "unable to stop unit %s", serviceName)
	}
	<-responseCh
	klog.FromContext(ctx).Info("service has been stopped", "unit", serviceName)
	return nil
}

func createService(ctx context.Context) error {
	if err := os.WriteFile(servicePath, []byte(agentServiceContent), 0644); err != nil {
		return err
	}
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	if err := conn.ReloadContext(ctx); err != nil {
		return errors.Wrapf(err, "unable to reload unit files")
	}
	_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, false)
	if err != nil {
		return err
	}

	responseCh := make(chan string)
	if _, err := conn.StartUnitContext(ctx, serviceName, "replace", responseCh); err != nil {
		return errors.Wrapf(err, "unable to start unit %s", serviceName)
	}

	response := <-responseCh
	fmt.Printf("service %q has been started. response: %s\n", serviceName, response)
	return nil
}

func saveFile(dst string, data []byte, dstMode os.FileMode, dirMode os.FileMode) (int, error) {
	if err := os.MkdirAll(filepath.Dir(dst), dirMode); err != nil {
		return 0, err
	}
	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, dstMode)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := destination.Write(data)
	return nBytes, err
}

func copyFile(dst, src string, dstMode os.FileMode) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, dstMode)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}
