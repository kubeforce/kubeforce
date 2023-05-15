/*
Copyright 2021 The Kubeforce Authors.

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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"k3f.io/kubeforce/agent/pkg/config"
	configutils "k3f.io/kubeforce/agent/pkg/config/utils"
	"k3f.io/kubeforce/agent/pkg/install"
)

// NewInitCommand returns a cobra command for install agent to the host..
func NewInitCommand() *cobra.Command {
	c := logsv1.NewLoggingConfiguration()
	klog.EnableContextualLogging(true)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Register and start the agent as a systemd service.",
		Long: `Register and start the agent as a systemd service.
`,
		Run: func(cmd *cobra.Command, args []string) {
			logs.InitLogs()
			if err := logsv1.ValidateAndApply(c, nil); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			ctx := klog.NewContext(ctrl.SetupSignalHandler(), klog.Background())
			err := runInitCmd(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	logsv1.AddFlags(c, cmd.Flags())
	return cmd
}

func init() {
	rootCmd.AddCommand(NewInitCommand())
}

func printConfig(cfg *config.Config) error {
	cfg = cfg.DeepCopy()
	fmt.Println("===============CONFIG================")
	if len(cfg.Spec.TLS.PrivateKeyData) > 0 {
		cfg.Spec.TLS.PrivateKeyData = []byte("--- REDACTED ---")
	}
	marshal, err := configutils.Marshal(cfg)
	if err != nil {
		return err
	}
	fmt.Println(string(marshal))
	fmt.Println("===============END CONFIG================")
	return nil
}

func runInitCmd(ctx context.Context) error {
	cfg, err := configutils.LoadFromFile(cfgFile)
	if err != nil {
		return errors.Wrapf(err, "unable to read config file: %q", cfgFile)
	}

	// print config to stdout
	if err := printConfig(cfg); err != nil {
		return err
	}

	if err := install.Install(ctx, *cfg); err != nil {
		return err
	}

	return nil
}
