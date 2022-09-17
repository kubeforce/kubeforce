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
	"fmt"
	"os"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"

	"k3f.io/kubeforce/agent/pkg/config"
	configutils "k3f.io/kubeforce/agent/pkg/config/utils"
)

// Uninstall uninstalls agent as the systemd service and removes data files.
func Uninstall(ctx context.Context, cfgSpec *config.ConfigSpec, wait bool) error {
	if cfgSpec == nil {
		cfg, err := findConfig()
		if err != nil {
			return err
		}
		cfgSpec = &cfg.Spec
	}
	if cfgSpec != nil {
		if err := os.RemoveAll(cfgSpec.Etcd.CertsDir); err != nil {
			return err
		}
		if err := os.RemoveAll(cfgSpec.Etcd.DataDir); err != nil {
			return err
		}
		if err := os.RemoveAll(cfgSpec.PlaybookPath); err != nil {
			return err
		}
		if err := os.RemoveAll(cfgSpec.TLS.CertFile); err != nil {
			return err
		}
		if err := os.RemoveAll(cfgSpec.TLS.PrivateKeyFile); err != nil {
			return err
		}
		if err := os.RemoveAll(cfgSpec.Authentication.X509.ClientCAFile); err != nil {
			return err
		}
	}
	// remove all internal data
	if err := os.RemoveAll("/var/lib/kubeforce/"); err != nil {
		return err
	}
	// remove binary
	if err := os.RemoveAll(agentPath); err != nil {
		return err
	}
	return removeService(ctx, wait)
}

func removeService(ctx context.Context, wait bool) error {
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}
	_, err = conn.DisableUnitFilesContext(ctx, []string{serviceName}, false)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(servicePath); err != nil {
		return err
	}
	if err := conn.ReloadContext(ctx); err != nil {
		return errors.Wrapf(err, "unable to reload unit files")
	}
	var responseCh chan string
	if wait {
		responseCh = make(chan string)
		defer close(responseCh)
	}

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", responseCh); err != nil {
		return errors.Wrapf(err, "unable to stop unit %s", serviceName)
	}
	if wait {
		response := <-responseCh
		fmt.Printf("service %q has been stoped. response: %s\n", serviceName, response)
	}
	return nil
}

func findConfig() (*config.Config, error) {
	_, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return configutils.LoadFromFile(configPath)
}
