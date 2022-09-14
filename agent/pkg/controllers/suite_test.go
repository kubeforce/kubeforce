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

package controllers

import (
	"context"
	"fmt"
	"os"
	"testing"

	clientset "k3f.io/kubeforce/agent/pkg/generated/clientset/versioned"

	"github.com/pkg/errors"
	"k3f.io/kubeforce/agent/pkg/apiserver"
	"k3f.io/kubeforce/agent/pkg/config"
	"k3f.io/kubeforce/agent/pkg/envtest"
	"k3f.io/kubeforce/agent/pkg/manager"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	restcfg      *rest.Config
	k8sClient    client.Client
	k8sClientset *clientset.Clientset
	ctx          context.Context
	cancel       context.CancelFunc
)

func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	if err := setup(ctx); err != nil {
		fmt.Println(errors.Cause(err))
		os.Exit(1)
	}
	code := m.Run()
	os.Exit(code)
}

func setup(ctx context.Context) error {
	fmt.Println("BeforeSuite")
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	testEnv := &envtest.Environment{}
	var err error
	restcfg, err = testEnv.Start(ctx, createCtrlManager)
	if err != nil {
		return errors.Wrap(err, "unable to start controller manager")
	}
	k8sClientset, err = clientset.NewForConfig(restcfg)
	if err != nil {
		return errors.Wrap(err, "unable to create a clientset")
	}
	k8sClient, err = client.New(restcfg, client.Options{Scheme: apiserver.Scheme})
	if err != nil {
		return errors.Wrap(err, "unable to get k8s client")
	}
	return nil
}

func createCtrlManager(agentConfig *config.Config, config *rest.Config) manager.RunnableFunc {
	return func(ctx context.Context) error {
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			Scheme: apiserver.Scheme,
		})
		if err != nil {
			return err
		}
		if err := (&PlaybookReconciler{
			PlaybookPath: agentConfig.Spec.PlaybookPath,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&PlaybookDeploymentReconciler{}).SetupWithManager(mgr); err != nil {
			return err
		}
		return mgr.Start(ctx)
	}
}
