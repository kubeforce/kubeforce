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

// Package controllers implements controller functionality.
package controllers

import (
	"context"
	"os"
	"time"

	certutil "github.com/cert-manager/cert-manager/pkg/api/util"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/prober"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/repository"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/secret"
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
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

// Reconcile reconciles KubeforceAgent object.
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

	log = log.WithValues("agent", client.ObjectKeyFromObject(kfAgent))

	// Fetch the Cluster.
	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.Client, kfAgent.ObjectMeta)
	if err != nil && errors.Cause(err) != capiutil.ErrNoCluster {
		log.Error(err, "unable to get cluster for KubeforceAgent", "playbook", req)
		return ctrl.Result{}, err
	}

	if cluster != nil {
		log = log.WithValues("cluster", cluster.Name)
	}

	// Return early if the object or Cluster is paused.
	if cluster != nil && cluster.Spec.Paused || annotations.HasPaused(kfAgent) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}
	ctx = ctrl.LoggerInto(ctx, log)

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
			conditions.MarkUnknown(kfAgent, infrav1.HealthyCondition, "UnknownProbeState", "")
		} else if probeStatus.ProbeResult {
			conditions.MarkTrue(kfAgent, infrav1.HealthyCondition)
		} else {
			conditions.MarkFalse(kfAgent, infrav1.HealthyCondition, infrav1.ProbeFailedReason, clusterv1.ConditionSeverityInfo, probeStatus.Message)
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

	return r.reconcileNormal(ctx, kfAgent)
}

// SetupWithManager will add watches for this controller.
func (r *KubeforceAgentReconciler) SetupWithManager(logger logr.Logger, mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceAgent{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{OwnerType: &infrav1.KubeforceAgent{}},
		).
		Watches(
			&source.Kind{Type: &certv1.Certificate{}},
			&handler.EnqueueRequestForOwner{OwnerType: &infrav1.KubeforceAgent{}},
		).
		Build(r)
	if err != nil {
		return err
	}
	clusterToAgents, err := capiutil.ClusterToObjectsMapper(mgr.GetClient(), &infrav1.KubeforceAgentList{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToAgents),
		predicates.ClusterUnpaused(logger),
	)
	if err != nil {
		return errors.Wrap(err, "failed to add Watch for Clusters to controller manager")
	}
	return nil
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
	log := ctrl.LoggerFrom(ctx)

	kfMachine, err := r.reconcileDeleteMachine(ctx, kfAgent)
	if err != nil {
		return ctrl.Result{}, err
	}
	if kfMachine != nil {
		return ctrl.Result{}, nil
	}
	playbooks, err := r.getPlaybooks(ctx, kfAgent.Namespace, kfAgent.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list Playbooks part of KubeforceAgent %s/%s", kfAgent.Namespace, kfAgent.Name)
	}
	if len(playbooks) > 0 {
		log.Info("Waiting for Playbooks to be deleted", "count", len(playbooks))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	pds, err := r.getPlaybookDeployments(ctx, kfAgent.Namespace, kfAgent.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list PlaybookDeployments part of KubeforceAgent %s/%s", kfAgent.Namespace, kfAgent.Name)
	}

	if len(pds) > 0 {
		log.Info("Waiting for PlaybookDeployments to be deleted", "count", len(pds))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if agent.IsHealthy(kfAgent) {
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
			log.Error(err, "unable to uninstall the agent from the machine")
			return ctrl.Result{}, err
		}
	}
	r.ProbeController.RemoveProbe(client.ObjectKeyFromObject(kfAgent).String())
	controllerutil.RemoveFinalizer(kfAgent, infrav1.AgentFinalizer)

	return ctrl.Result{}, nil
}

func (r *KubeforceAgentReconciler) getPlaybooks(ctx context.Context, namespace, agentName string) ([]infrav1.Playbook, error) {
	ml := &infrav1.PlaybookList{}
	if err := r.Client.List(
		ctx,
		ml,
		client.InNamespace(namespace),
		client.MatchingLabels{
			infrav1.PlaybookAgentNameLabelName: agentName,
		},
	); err != nil {
		return nil, errors.Wrap(err, "failed to list PlaybookList")
	}

	return ml.Items, nil
}

func (r *KubeforceAgentReconciler) getPlaybookDeployments(ctx context.Context, namespace, agentName string) ([]infrav1.PlaybookDeployment, error) {
	ml := &infrav1.PlaybookDeploymentList{}
	if err := r.Client.List(
		ctx,
		ml,
		client.InNamespace(namespace),
		client.MatchingLabels{
			infrav1.PlaybookAgentNameLabelName: agentName,
		},
	); err != nil {
		return nil, errors.Wrap(err, "failed to list PlaybookDeploymentList")
	}

	return ml.Items, nil
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

	if !kfMachine.DeletionTimestamp.IsZero() {
		return kfMachine, nil
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
	// wait until the agent is ready to connect
	if !agent.IsHealthy(kfAgent) {
		return ctrl.Result{}, nil
	}
	changedTLSCert, err := r.syncAgentTLSSecret(ctx, kfAgent, true)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changedTLSCert {
		kfAgent.Status.AgentInfo = nil
		return ctrl.Result{}, err
	}
	changedClientCA, err := r.syncAgentClientSecret(ctx, kfAgent, true)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changedClientCA {
		kfAgent.Status.AgentInfo = nil
		return ctrl.Result{}, err
	}
	if err := r.reconcileAgentInfo(ctx, kfAgent); err != nil {
		log.Error(err, "unable to get the agent info")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *KubeforceAgentReconciler) syncAgentTLSSecret(ctx context.Context, kfAgent *infrav1.KubeforceAgent, needUpload bool) (bool, error) {
	issuedTLSKey := agent.GetAgentTLSObjectKey(kfAgent, agent.IssuedKey)
	issuedTLSSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, issuedTLSKey, issuedTLSSecret); err != nil {
		return false, errors.Wrapf(err, "failed to get tls secret %s", issuedTLSKey)
	}
	activeTLSKey := agent.GetAgentTLSObjectKey(kfAgent, agent.ActiveKey)
	activeTLSSecret := &corev1.Secret{}
	needCreate := false
	if err := r.Client.Get(ctx, activeTLSKey, activeTLSSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, errors.Wrapf(err, "failed to get tls secret %s", activeTLSKey)
		}
		needCreate = true
	}

	patchObj := client.MergeFrom(activeTLSSecret.DeepCopy())
	r.copySecretFields(activeTLSSecret, issuedTLSSecret)
	changed, err := patchutil.HasChanges(patchObj, activeTLSSecret)
	if err != nil {
		return false, errors.WithStack(err)
	}
	if changed && needUpload {
		clientset, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
		if err != nil {
			return false, errors.WithStack(err)
		}
		mode := os.FileMode(0o600)
		if err := clientset.UploadData(ctx, "/etc/kubeforce/certs/tls.crt", activeTLSSecret.Data[corev1.TLSCertKey], &mode); err != nil {
			return false, errors.Wrapf(err, "unable to upload the tls certificate")
		}
		if err := clientset.UploadData(ctx, "/etc/kubeforce/certs/tls.key", activeTLSSecret.Data[corev1.TLSPrivateKeyKey], &mode); err != nil {
			return false, errors.Wrapf(err, "unable to upload the tls private key")
		}
	}

	if needCreate {
		activeTLSSecret.SetName(activeTLSKey.Name)
		activeTLSSecret.SetNamespace(activeTLSKey.Namespace)
		if err := r.Client.Create(ctx, activeTLSSecret); err != nil {
			return false, errors.Wrapf(err, "unable to create tls secret %s", activeTLSKey)
		}
		r.AgentClientCache.DeleteHolder(client.ObjectKeyFromObject(kfAgent))
		return true, nil
	}

	if changed {
		err := r.Client.Patch(ctx, activeTLSSecret, patchObj)
		if err != nil {
			return false, errors.Wrapf(err, "failed to patch Secret %s", activeTLSKey)
		}
		r.AgentClientCache.DeleteHolder(client.ObjectKeyFromObject(kfAgent))
	}

	return changed, nil
}

func (r *KubeforceAgentReconciler) syncAgentClientSecret(ctx context.Context, kfAgent *infrav1.KubeforceAgent, needUpload bool) (bool, error) {
	issuedClientKey, err := agent.GetAgentClientCertObjectKey(kfAgent, agent.IssuedKey)
	if err != nil {
		return false, err
	}
	issuedClientSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, *issuedClientKey, issuedClientSecret); err != nil {
		return false, errors.Wrapf(err, "failed to get tls secret %s", issuedClientKey.String())
	}
	activeClientKey, err := agent.GetAgentClientCertObjectKey(kfAgent, agent.ActiveKey)
	if err != nil {
		return false, err
	}
	activeClientSecret := &corev1.Secret{}
	needCreate := false
	if err := r.Client.Get(ctx, *activeClientKey, activeClientSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, errors.Wrapf(err, "failed to get tls secret %s", activeClientKey.String())
		}
		needCreate = true
	}

	patchObj := client.MergeFrom(activeClientSecret.DeepCopy())
	r.copySecretFields(activeClientSecret, issuedClientSecret)
	controllerOwnerRef := *metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent"))
	activeClientSecret.OwnerReferences = []metav1.OwnerReference{controllerOwnerRef}
	changed, err := patchutil.HasChanges(patchObj, activeClientSecret)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if changed && needUpload {
		clientset, err := r.AgentClientCache.GetClientSet(ctx, client.ObjectKeyFromObject(kfAgent))
		if err != nil {
			return false, errors.WithStack(err)
		}
		mode := os.FileMode(0o600)
		if err := clientset.UploadData(ctx, "/etc/kubeforce/certs/client-ca.crt", activeClientSecret.Data[secret.TLSCAKey], &mode); err != nil {
			return false, errors.Wrapf(err, "unable to upload the tls certificate")
		}
	}

	if needCreate {
		activeClientSecret.SetName(activeClientKey.Name)
		activeClientSecret.SetNamespace(activeClientKey.Namespace)
		if err := r.Client.Create(ctx, activeClientSecret); err != nil {
			return false, errors.Wrapf(err, "unable to create tls secret %s", activeClientKey)
		}
		r.AgentClientCache.DeleteHolder(client.ObjectKeyFromObject(kfAgent))
		return true, nil
	}

	if changed {
		err := r.Client.Patch(ctx, activeClientSecret, patchObj)
		if err != nil {
			return false, errors.Wrapf(err, "failed to patch Secret %s", activeClientKey)
		}
		r.AgentClientCache.DeleteHolder(client.ObjectKeyFromObject(kfAgent))
	}

	return changed, nil
}

func (r *KubeforceAgentReconciler) reconcileAgentInfo(ctx context.Context, kfAgent *infrav1.KubeforceAgent) error {
	if !agent.IsHealthy(kfAgent) || kfAgent.Status.AgentInfo != nil {
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
	if _, err := r.syncAgentTLSSecret(ctx, kfAgent, false); err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.AgentInstallingFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	if _, err := r.syncAgentClientSecret(ctx, kfAgent, false); err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.AgentInstallingFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	agentHelper, err := agent.GetHelper(ctx, r.Client, r.Storage, kfAgent)
	if err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.AgentInstallingFailedReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{}, err
	}
	_, err = agentHelper.GetSSHConfig(ctx)
	if err != nil {
		conditions.MarkFalse(kfAgent, infrav1.AgentInstalledCondition, infrav1.WaitingForSSHConfigurationReason, clusterv1.ConditionSeverityInfo, err.Error())
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}
	kfAgent.Status.AgentInfo = nil
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

func (r *KubeforceAgentReconciler) copySecretFields(dst *corev1.Secret, src *corev1.Secret) {
	if dst.Data == nil {
		dst.Data = make(map[string][]byte)
	}
	for key, data := range src.Data {
		dst.Data[key] = data
	}
	if dst.Labels == nil {
		dst.Labels = make(map[string]string)
	}
	for key, val := range src.Labels {
		dst.Labels[key] = val
	}
	dst.OwnerReferences = src.OwnerReferences
}

func (r *KubeforceAgentReconciler) reconcileTLSCert(ctx context.Context, kfAgent *infrav1.KubeforceAgent) (ctrl.Result, error) {
	if kfAgent.Spec.Config.CertTemplate.IssuerRef.Name == "" {
		msg := "Waiting for the certification issuer reference to be specified"
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssuerRefReason, clusterv1.ConditionSeverityError, msg)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	certKey := agent.GetAgentTLSObjectKey(kfAgent, agent.IssuedKey)

	cert := &certv1.Certificate{}
	controllerOwnerRef := *metav1.NewControllerRef(kfAgent, infrav1.GroupVersion.WithKind("KubeforceAgent"))
	if err := r.Client.Get(ctx, certKey, cert); err != nil {
		if apierrors.IsNotFound(err) {
			createErr := r.createAgentServCertificate(ctx, certKey, kfAgent, controllerOwnerRef)
			if createErr != nil {
				conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssueReason, clusterv1.ConditionSeverityError, createErr.Error())
				return ctrl.Result{}, createErr
			}
			return ctrl.Result{}, nil
		}
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssueReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	cond := certutil.GetCertificateCondition(cert, certv1.CertificateConditionReady)
	if cond == nil || cond.Status != cmmeta.ConditionTrue || cond.ObservedGeneration != cert.Generation {
		conditions.MarkFalse(kfAgent, infrav1.AgentTLSCondition, infrav1.WaitingForCertIssueReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
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
	dnsNames := agent.Spec.Config.CertTemplate.DNSNames
	dnsNames = append(dnsNames, stringutil.Filter(stringutil.IsNotEmpty, agent.Spec.Addresses.ExternalDNS, agent.Spec.Addresses.InternalDNS)...)
	ipAddresses := agent.Spec.Config.CertTemplate.IPAddresses
	ipAddresses = append(ipAddresses, stringutil.Filter(stringutil.IsNotEmpty, agent.Spec.Addresses.ExternalIP, agent.Spec.Addresses.InternalIP)...)
	cert := &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Spec: certv1.CertificateSpec{
			CommonName:  certKey.Name,
			Duration:    agent.Spec.Config.CertTemplate.Duration,
			RenewBefore: agent.Spec.Config.CertTemplate.RenewBefore,
			DNSNames:    dnsNames,
			IPAddresses: ipAddresses,
			SecretName:  certKey.Name,
			IssuerRef: cmmeta.ObjectReference{
				Name:  agent.Spec.Config.CertTemplate.IssuerRef.Name,
				Kind:  agent.Spec.Config.CertTemplate.IssuerRef.Kind,
				Group: agent.Spec.Config.CertTemplate.IssuerRef.Group,
			},
		},
	}
	if agent.Spec.Config.CertTemplate.PrivateKey != nil {
		cert.Spec.PrivateKey = &certv1.CertificatePrivateKey{
			RotationPolicy: certv1.PrivateKeyRotationPolicy(agent.Spec.Config.CertTemplate.PrivateKey.RotationPolicy),
			Encoding:       certv1.PrivateKeyEncoding(agent.Spec.Config.CertTemplate.PrivateKey.Encoding),
			Algorithm:      certv1.PrivateKeyAlgorithm(agent.Spec.Config.CertTemplate.PrivateKey.Algorithm),
			Size:           agent.Spec.Config.CertTemplate.PrivateKey.Size,
		}
	}
	err := r.Client.Create(ctx, cert)
	if err != nil {
		return errors.Wrap(err, "unable to create certificate")
	}
	return nil
}

func patchKubeforceAgent(ctx context.Context, patchHelper *patch.Helper, agent *infrav1.KubeforceAgent) error {
	conditions.SetSummary(agent,
		conditions.WithConditions(
			infrav1.AgentInstalledCondition,
			infrav1.HealthyCondition,
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
			infrav1.HealthyCondition,
			infrav1.AgentTLSCondition,
		}},
	)
}
