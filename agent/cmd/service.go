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

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k3f.io/kubeforce/agent/pkg/apiserver"
	"k3f.io/kubeforce/agent/pkg/config"
	configutils "k3f.io/kubeforce/agent/pkg/config/utils"
	"k3f.io/kubeforce/agent/pkg/controllers"
	"k3f.io/kubeforce/agent/pkg/manager"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// ServiceCmd represents the Service command
var ServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run the agent as service.",
	Long:  `Run the agent as service.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runServiceCmd(); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(ServiceCmd)
}

func runServiceCmd() error {
	cfg, err := configutils.LoadFromFile(cfgFile)
	if err != nil {
		return errors.Wrapf(err, "unable to read config file: %q", cfgFile)
	}
	// print config to stdout
	if err := printConfig(cfg); err != nil {
		return err
	}

	log := ctrlzap.NewRaw(ctrlzap.UseDevMode(true), ctrlzap.Level(zapcore.InfoLevel))
	zap.ReplaceGlobals(log)
	logger := zapr.NewLogger(log)
	ctrl.SetLogger(logger)
	scheme := apiserver.Scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	mgr, err := manager.NewManager(cfg.Spec.ShutdownGracePeriod.Duration)
	if err != nil {
		return err
	}
	etcdSrv, err := apiserver.NewEtcdServer(cfg.Spec.Etcd)
	if err != nil {
		return err
	}
	mgr.Add(etcdSrv)
	srv, err := apiserver.NewServer(cfg.Spec)
	if err != nil {
		return err
	}
	mgr.Add(srv)
	mgr.Add(createInternalCtrlManager(cfg, srv.LoopbackClientConfig))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		return err
	}
	return nil
}

func createInternalCtrlManager(agentConfig *config.Config, config *rest.Config) manager.RunnableFunc {
	return func(ctx context.Context) error {
		scheme := apiserver.Scheme
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			Scheme: scheme,
		})
		if err != nil {
			return err
		}
		if err := (&controllers.PlaybookReconciler{
			PlaybookPath: agentConfig.Spec.PlaybookPath,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&controllers.PlaybookDeploymentReconciler{}).SetupWithManager(mgr); err != nil {
			return err
		}
		return mgr.Start(ctx)
	}
}
