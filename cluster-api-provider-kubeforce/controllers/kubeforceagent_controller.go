/*
Copyright 2020 The Kubernetes Authors.

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

// Package controllers implements controller functionality.
package controllers

import (
	"context"
	"time"

	certutil "github.com/cert-manager/cert-manager/pkg/api/util"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/prober"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/repository"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KubeforceAgentReconciler reconciles a KubeforceAgent object.
type KubeforceAgentReconciler struct {
	Client           client.Client
	Storage          *repository.Storage
	ProbeController  prober.Controller
	AgentClientCache *agentctrl.ClientCache
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceagents/status;kubeforceagents/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *KubeforceAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, rerr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the KubeforceAgent instance.
	kfAgent := &infrav1.KubeforceAgent{}
	if err := r.Client.Get(ctx, req.NamespacedName, kfAgent); err != nil {
		if apierrors.IsNotFound(err) {
			r.ProbeController.RemoveProbe(req.NamespacedName.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log = log.WithValues("agent", kfAgent.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kfAgent, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the KubeforceAgent object and status after each reconciliation.
	defer func() {
		r.reconcilePhase(ctx, kfAgent)
		if err := patchKubeforceAgent(ctx, patchHelper, kfAgent); err != nil {
			log.Error(err, "failed to patch KubeforceAgent")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	if kfAgent.Spec.Installed {
		objectKey := client.ObjectKeyFromObject(kfAgent)
		params := prober.ProbeParams{
			TimeoutSeconds:   4,
			PeriodSeconds:    5,
			SuccessThreshold: 2,
			FailureThreshold: 2,
		}
		r.ProbeController.EnsureProbe(ctx, NewAgentProbeHandler(objectKey, r.Client, r.AgentClientCache), params)
		probeStatus := r.ProbeController.GetCurrentStatus(objectKey.String())
		if probeStatus == nil {
			conditions.MarkUnknown(kfAgent, infrav1.Healthy, "UnknownProbeState", "")
		} else if probeStatus.ProbeResult {
			conditions.MarkTrue(kfAgent, infrav1.Healthy)
		} else {
			conditions.MarkFalse(kfAgent, infrav1.Healthy, infrav1.ProbeFailedReason, clusterv1.ConditionSeverityInfo, probeStatus.Message)
		}
	}

	if !kfAgent.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, kfAgent)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(kfAgent, infrav1.AgentFinalizer) {
		controllerutil.AddFinalizer(kfAgent, infrav1.AgentFinalizer)
		return ctrl.Result{}, nil
	}

	// Add foregroundDeletion finalizer to KubeforceAgent
	if !controllerutil.ContainsFinalizer(kfAgent, metav1.FinalizerDeleteDependents) {
		controllerutil.AddFinalizer(kfAgent, metav1.FinalizerDeleteDependents)
	}

	return r.reconcileNormal(ctx, kfAgent)
}

// SetupWithManager will add watches for this controller.
func (r *KubeforceAgentReconciler) SetupWithManager(logger logr.Logger, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceAgent{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{OwnerType: &infrav1.KubeforceAgent{}},
		).
		Complete(r)
}

func (r *KubeforceAgentReconciler) reconcilePhase(_ context.Context, a *infrav1.KubeforceAgent) {
	// Set the phase to "pending" if nil.
	if a.Status.Phase == "" {
		a.Status.Phase = infrav1.AgentPhasePending
	}

	// Set the phase to "provisioning" if tls certs is ready and the infrastructure isn't.
	if !a.Spec.Installed && conditions.IsTrue(a, infrav1.AgentTLSCondition) {
		a.Status.Phase = infrav1.AgentPhaseProvisioning
	}

	if a.Spec.Installed && !conditions.IsTrue(a, infrav1.AgentInstalledCondition) {
		a.Status.Phase = infrav1.AgentPhaseInstalled
	}

	if a.Spec.Installed && conditions.IsTrue(a, infrav1.AgentInstalledCondition) {
		a.Status.Phase = infrav1.AgentPhaseRunning
	}

	// Set the phase to "failed" if any of Status.FailureReason or Status.FailureMessage is not-nil.
	if a.Status.FailureReason != "" || a.Status.FailureMessage != "" {
		a.Status.Phase = infrav1.AgentPhaseFailed
	}

	// Set the phase to "deleting" if the deletion timestamp is set.
	if !a.DeletionTimestamp.IsZero() {
		a.Status.Phase = infrav1.AgentPhaseDeleting
	}
}

func (r *KubeforceAgentReconciler) reconcileDelete(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(kfAgent, infrav1.AgentFinalizer) {
		return ctrl.Result{}, nil
	}
	obj, err := r.reconcileDeleteMachine(ctx, kfAgent)
	if err != nil {
		return ctrl.Result{}, err
	}
	if obj == nil {
		if agent.IsReady(kfAgent) {
			objectKey := client.ObjectKeyFromObject(kfAgent)
			clientset, err := r.AgentClientCache.GetClientSet(ctx, objectKey)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = clientset.RESTClient().Delete().
				AbsPath("uninstall").
				Do(ctx).
				Error()
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "unable to uninstall the agent from the machine")
				return ctrl.Result{}, err
			}
		}
		r.ProbeController.RemoveProbe(client.ObjectKeyFromObject(kfAgent).String())
		controllerutil.RemoveFinalizer(kfAgent, infrav1.AgentFinalizer)
	}
	return ctrl.Result{}, nil
}

// reconcileDeleteExternal tries to delete external references.
func (r *KubeforceAgentReconciler) reconcileDeleteMachine(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (*infrav1.KubeforceMachine, error) {
	kfMachineName := kfAgent.Labels[infrav1.AgentMachineLabel]
	if kfMachineName == "" {
		return nil, nil
	}
	key := client.ObjectKey{
		Namespace: kfAgent.Namespace,
		Name:      kfMachineName,
	}
	kfMachine := &infrav1.KubeforceMachine{}

	err := r.Client.Get(ctx, key, kfMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to get KubeforceMachine %q", key)
	}

	if err := r.Client.Delete(ctx, kfMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to delete KubeforceMachine %q", key)
	}
	return kfMachine, nil
}

func (r *KubeforceAgentReconciler) reconcileNormal(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	// Generate tls certificate for the agent if needed
	if result, err := r.reconcileTLSCert(ctx, kfAgent); !result.IsZero() || err != nil {
		if err != nil {
			log.Error(err, "failed to reconcile agent tls certificate")
		}
		return result, err
	}
	if result, err := r.reconcileAgentInstallation(ctx, kfAgent); !result.IsZero() || err != nil {
		if err != nil {
			log.Error(err, "failed to reconcile the agent installation")
		}
		return result, err
	}
	if err := r.reconcileAgentInfo(ctx, kfAgent); err != nil {
		log.Error(err, "unable to get the agent info")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *KubeforceAgentReconciler) reconcileAgentInfo(ctx context.Context, kfAgent *infrav1.KubeforceAgent) error {
	if !agent.IsReady(kfAgent) || kfAgent.Status.AgentInfo != nil {
		return nil
	}
	clientset, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
	if err != nil {
		return err
	}
	v, err := clientset.ServerVersion()
	if err != nil {
		return err
	}
	kfAgent.Status.AgentInfo = &infrav1.AgentInfo{
		Version:   v.GitVersion,
		GitCommit: v.GitCommit,
		Platform:  v.Platform,
		BuildDate: v.BuildDate,
	}
	return nil
}

func (r *KubeforceAgentReconciler) reconcileAgentInstallation(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (ctrl.Result, error) {
	if kfAgent.Spec.Installed {
		if !conditions.IsTrue(kfAgent, infrav1.AgentInstalledCondition) {
			conditions.MarkTrue(kfAgent, infrav1.AgentInstalledCondition)
		}
		return ctrl.Result{}, nil
	}
	if kfAgent.Spec.SSH.SecretName == "" {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.WaitingForSSHConfigurationReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}
	agentHelper, err := agent.GetHelper(ctx, r.Client, r.Storage, kfAgent)
	if err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.AgentInstallingFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	_, err = agentHelper.GetSshConfig(ctx)
	if err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.WaitingForSSHConfigurationReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	err = agentHelper.Install(ctx)
	if err != nil {
		kfAgent.Status.FailureReason = infrav1.InstallAgentError
		kfAgent.Status.FailureMessage = err.Error()
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.AgentInstallingFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
	}
	kfAgent.Spec.Installed = true
	if kfAgent.Status.FailureReason == infrav1.InstallAgentError {
		kfAgent.Status.FailureReason = ""
		kfAgent.Status.FailureMessage = ""
	}
	conditions.MarkTrue(kfAgent, infrav1.AgentInstalledCondition)
	return ctrl.Result{}, nil
}

func (r *KubeforceAgentReconciler) reconcileTLSCert(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if kfAgent.Spec.Config.CertIssuerRef.Name == "" || kfAgent.Spec.Config.CertIssuerRef.Kind == "" {
		log.Info("Waiting for the certification issuer reference to be initialized")
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssuerRefReason, clusterv1.ConditionSeverityError, "")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	certKey := agent.GetAgentTLSObjectKey(kfAgent)

	cert := &certv1.Certificate{}
	controllerOwnerRef := *metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent"))
	// TODO: certificate must be reissued if CertIssuerRef is changed
	if err := r.Client.Get(ctx, certKey, cert); err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssueReason, clusterv1.ConditionSeverityInfo, "")
		if apierrors.IsNotFound(err) {
			createErr := r.createAgentServCertificate(ctx, certKey, kfAgent, controllerOwnerRef)
			if createErr != nil {
				return ctrl.Result{}, createErr
			}
			return ctrl.Result{
				RequeueAfter: 5 * time.Second,
			}, nil
		}
		return ctrl.Result{}, err
	}
	cond := certutil.GetCertificateCondition(cert, cmapi.CertificateConditionReady)
	if cond == nil || cond.Status != cmmeta.ConditionTrue || cond.ObservedGeneration != cert.Generation {
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssueReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
		}, nil
	}

	s := &corev1.Secret{}
	if err := r.Client.Get(ctx, certKey, s); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get tls secret %s", certKey.Name)
	}
	if r.shouldAdopt(s) {
		patchObj := client.MergeFrom(s.DeepCopy())
		s.OwnerReferences = capiutil.EnsureOwnerRef(s.OwnerReferences, controllerOwnerRef)
		if err := r.Client.Patch(ctx, s, patchObj); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to patch secret %s", certKey)
		}
	}
	conditions.MarkTrue(kfAgent, infrav1.AgentTLSCondition)
	return ctrl.Result{}, nil
}

func (r *KubeforceAgentReconciler) shouldAdopt(s *corev1.Secret) bool {
	return !capiutil.HasOwner(s.OwnerReferences, infrav1.GroupVersion.String(), []string{"KubeforceAgent"})
}

func (r *KubeforceAgentReconciler) createAgentServCertificate(ctx context.Context, certKey client.ObjectKey, agent *infrav1.KubeforceAgent, owner metav1.OwnerReference) error {
	cert := &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Spec: certv1.CertificateSpec{
			CommonName: certKey.Name,
			Duration: &metav1.Duration{
				Duration: time.Hour * 24 * 365,
			},
			DNSNames:    stringutil.Filter(stringutil.IsNotEmpty, agent.Spec.Addresses.ExternalDNS, agent.Spec.Addresses.InternalDNS),
			IPAddresses: stringutil.Filter(stringutil.IsNotEmpty, agent.Spec.Addresses.ExternalIP, agent.Spec.Addresses.InternalIP),
			SecretName:  certKey.Name,
			IssuerRef: cmmeta.ObjectReference{
				Name:  agent.Spec.Config.CertIssuerRef.Name,
				Kind:  agent.Spec.Config.CertIssuerRef.Kind,
				Group: certv1.SchemeGroupVersion.Group,
			},
		},
	}
	err := r.Client.Create(ctx, cert)
	if err != nil {
		return err
	}
	return nil
}

func patchKubeforceAgent(ctx context.Context, patchHelper *patch.Helper, agent *infrav1.KubeforceAgent) error {
	conditions.SetSummary(agent,
		conditions.WithConditions(
			infrav1.AgentInstalledCondition,
			infrav1.Healthy,
			infrav1.AgentTLSCondition,
		),
		conditions.WithStepCounterIf(agent.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		agent,
		patch.WithStatusObservedGeneration{},
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.AgentInstalledCondition,
			infrav1.Healthy,
			infrav1.AgentTLSCondition,
		}},
	)
}
