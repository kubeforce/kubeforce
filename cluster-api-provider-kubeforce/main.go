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

package main

import (
	"flag"
	"os"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/prober"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/repository"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(expv1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(certv1.AddToScheme(scheme))

	utilruntime.Must(infrav1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "40dca779.cluster.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clusterCacheLog := logger.WithName("remote").WithName("ClusterCacheTracker")
	tracker, err := remote.NewClusterCacheTracker(
		mgr,
		remote.ClusterCacheTrackerOptions{
			Log:     &clusterCacheLog,
			Indexes: remote.DefaultIndexes,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to create cluster cache tracker")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	agentClientCache, err := agentctrl.NewClientCache(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create agent client cache")
		os.Exit(1)
	}
	storage := repository.NewStorage(logger.WithName("storage"), "/var/lib/kubeforce/storage")
	if err = (&agentctrl.CacheReconciler{
		Client:      mgr.GetClient(),
		ClientCache: agentClientCache,
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClientCache")
		os.Exit(1)
	}
	probeController := prober.NewController(logger.WithName("probe.controller"))
	if err := mgr.Add(probeController); err != nil {
		os.Exit(1)
	}

	if err = (&controllers.HTTPRepositoryReconciler{
		Client:  mgr.GetClient(),
		Storage: storage,
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HTTPRepository")
		os.Exit(1)
	}

	if err = (&controllers.KubeforceAgentReconciler{
		Client:           mgr.GetClient(),
		Storage:          storage,
		ProbeController:  probeController,
		AgentClientCache: agentClientCache,
	}).SetupWithManager(logger, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KubeforceAgent")
		os.Exit(1)
	}
	if err = (&controllers.KubeforceAgentGroupReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(logger, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KubeforceAgentGroup")
		os.Exit(1)
	}

	if err = (&controllers.KubeforceClusterReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("kf-cluster-controller"),
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KubeforceCluster")
		os.Exit(1)
	}
	if err = (&controllers.KubeforceMachineReconciler{
		Client:           mgr.GetClient(),
		Log:              logger.WithName("kf-machine-controller"),
		Tracker:          tracker,
		AgentClientCache: agentClientCache,
	}).SetupWithManager(logger, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KubeforceMachine")
		os.Exit(1)
	}
	if err = (&controllers.KubeforceMachinePoolReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("kf-machine-pool-controller"),
	}).SetupWithManager(logger, mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KubeforceMachinePool")
		os.Exit(1)
	}
	if err = (&controllers.PlaybookReconciler{
		Client:           mgr.GetClient(),
		Log:              logger.WithName("playbook-controller"),
		AgentClientCache: agentClientCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Playbook")
		os.Exit(1)
	}
	if err = (&controllers.PlaybookDeploymentReconciler{
		Client:           mgr.GetClient(),
		Log:              logger.WithName("playbookdeployment-controller"),
		AgentClientCache: agentClientCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PlaybookDeployment")
		os.Exit(1)
	}

	if err := mgr.Add(&controllers.Initializer{
		Log:    logger.WithName("initializer"),
		Client: mgr.GetClient(),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "initializer")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
