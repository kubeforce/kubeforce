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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/names"
	patchutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/patch"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
)

// KubeforceClusterReconciler reconciles a KubeforceCluster object.
type KubeforceClusterReconciler struct {
	Client client.Client
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kubeforceclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;update;patch;watch
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *KubeforceClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the KubeforceCluster instance
	kubeforceCluster := &infrav1.KubeforceCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, kubeforceCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kubeforceCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on KubeforceCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, kubeforceCluster) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(kubeforceCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the KubeforceCluster object and status after each reconciliation.
	defer func() {
		if err := patchKubeforceCluster(ctx, patchHelper, kubeforceCluster); err != nil {
			log.Error(err, "failed to patch KubeforceCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(kubeforceCluster, infrav1.ClusterFinalizer) {
		controllerutil.AddFinalizer(kubeforceCluster, infrav1.ClusterFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !kubeforceCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, kubeforceCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, cluster, kubeforceCluster)
}

func patchKubeforceCluster(ctx context.Context, patchHelper *patch.Helper, kubeforceCluster *infrav1.KubeforceCluster) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process (instead we are hiding it during the deletion process).
	conditions.SetSummary(kubeforceCluster,
		conditions.WithConditions(
			infrav1.LoadBalancerAvailableCondition,
		),
		conditions.WithStepCounterIf(kubeforceCluster.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		kubeforceCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.LoadBalancerAvailableCondition,
		}},
	)
}

func (r *KubeforceClusterReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, kubeforceCluster *infrav1.KubeforceCluster) (ctrl.Result, error) {
	if kubeforceCluster.Spec.Loadbalancer != nil && kubeforceCluster.Spec.Loadbalancer.Disabled {
		kubeforceCluster.Status.Ready = true
		conditions.MarkTrue(kubeforceCluster, infrav1.LoadBalancerAvailableCondition)
		return ctrl.Result{}, nil
	}
	if err := r.reconcileLoadbalancer(ctx, cluster, kubeforceCluster); err != nil {
		conditions.MarkFalse(kubeforceCluster, infrav1.LoadBalancerAvailableCondition, infrav1.LoadBalancerProvisioningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return ctrl.Result{}, err
	}
	kubeforceCluster.Status.Ready = true
	conditions.MarkTrue(kubeforceCluster, infrav1.LoadBalancerAvailableCondition)
	return ctrl.Result{}, nil
}

func (r *KubeforceClusterReconciler) reconcileLoadbalancer(ctx context.Context, cluster *clusterv1.Cluster, kubeforceCluster *infrav1.KubeforceCluster) error {
	machines, err := r.getControlPlaneMachineList(ctx, kubeforceCluster)
	if err != nil {
		return err
	}
	r.refreshAPIServers(ctx, kubeforceCluster, machines)
	if err := r.reconcileLBConfigMap(ctx, cluster, machines, kubeforceCluster); err != nil {
		return err
	}
	if err := r.reconcileLBDeployment(ctx, cluster, kubeforceCluster); err != nil {
		return err
	}
	if err := r.reconcileLBService(ctx, cluster, kubeforceCluster); err != nil {
		return err
	}
	if err := r.reconcileControlPlaneEndpoint(ctx, cluster, kubeforceCluster); err != nil {
		return err
	}
	return nil
}

func (r *KubeforceClusterReconciler) getExternalAddresses(ctx context.Context, machines []infrav1.KubeforceMachine) ([]string, error) {
	adresses := make([]string, 0, len(machines))
	for _, kfMachine := range machines {
		if kfMachine.Spec.AgentRef != nil {
			kfAgent := &infrav1.KubeforceAgent{}
			if err := r.Client.Get(ctx, client.ObjectKey{
				Namespace: kfMachine.Namespace,
				Name:      kfMachine.Spec.AgentRef.Name,
			}, kfAgent); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return nil, errors.Wrapf(err, "unable to get agent for machine %s", kfMachine.Name)
			}
			address := stringutil.Find(stringutil.IsNotEmpty, kfAgent.Spec.Addresses.ExternalDNS, kfAgent.Spec.Addresses.ExternalIP)
			adresses = append(adresses, address)
		}
	}
	return adresses, nil
}

func (r *KubeforceClusterReconciler) reconcileLBDeployment(ctx context.Context, cluster *clusterv1.Cluster,
	kubeforceCluster *infrav1.KubeforceCluster) error {
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	d := &appsv1.Deployment{}
	created := true
	err := r.Client.Get(ctx, key, d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			created = false
		} else {
			return err
		}
	}
	patchObj := client.MergeFrom(d.DeepCopy())
	d.Name = key.Name
	d.Namespace = key.Namespace
	r.fillLBDeployment(d, kubeforceCluster, cluster)
	if !created {
		if err := r.Client.Create(ctx, d); err != nil {
			return err
		}
		return nil
	}
	changed, err := patchutil.HasChanges(patchObj, d)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		diff, err := patchObj.Data(d)
		if err != nil {
			return err
		}
		r.Log.Info("updating loadbalancer Deployment", "key", key, "diff", string(diff))
		err = r.Client.Patch(ctx, d, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch Deployment")
		}
	}
	return nil
}

func (r *KubeforceClusterReconciler) reconcileLBConfigMap(ctx context.Context, cluster *clusterv1.Cluster,
	controlPlaneMachines []infrav1.KubeforceMachine, kubeforceCluster *infrav1.KubeforceCluster) error {
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      r.lbConfigName(cluster.Name),
	}
	cm := &corev1.ConfigMap{}
	created := true
	err := r.Client.Get(ctx, key, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			created = false
		} else {
			return err
		}
	}
	patchObj := client.MergeFrom(cm.DeepCopy())
	cm.Name = key.Name
	cm.Namespace = key.Namespace
	if err := r.fillLBConfigMap(ctx, cm, controlPlaneMachines, cluster, kubeforceCluster); err != nil {
		return err
	}
	if !created {
		if err := r.Client.Create(ctx, cm); err != nil {
			return err
		}
		return nil
	}
	changed, err := patchutil.HasChanges(patchObj, cm)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		r.Log.Info("updating loadbalancer ConfigMap", "key", key)
		err := r.Client.Patch(ctx, cm, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch ConfigMap")
		}
	}
	return nil
}

func (r *KubeforceClusterReconciler) reconcileLBService(ctx context.Context, cluster *clusterv1.Cluster,
	kubeforceCluster *infrav1.KubeforceCluster) error {
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	svc := &corev1.Service{}
	created := true
	err := r.Client.Get(ctx, key, svc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			created = false
		} else {
			return err
		}
	}
	patchObj := client.MergeFrom(svc.DeepCopy())
	svc.Name = key.Name
	svc.Namespace = key.Namespace
	r.fillLBService(svc, cluster, kubeforceCluster)
	if !created {
		if err := r.Client.Create(ctx, svc); err != nil {
			return err
		}
		return nil
	}
	changed, err := patchutil.HasChanges(patchObj, svc)
	if err != nil {
		return errors.WithStack(err)
	}

	if changed {
		r.Log.Info("updating loadbalancer Service", "key", key)
		err := r.Client.Patch(ctx, svc, patchObj)
		if err != nil {
			return errors.Wrapf(err, "failed to patch Service")
		}
	}
	return nil
}

//go:embed traefik.yaml
var traefikConfig string

var traefikConfigTmpl = template.Must(template.New("config").Parse(traefikConfig))

func (r *KubeforceClusterReconciler) fillLBConfigMap(ctx context.Context, cm *corev1.ConfigMap,
	controlPlaneMachines []infrav1.KubeforceMachine, cluster *clusterv1.Cluster, kubeforceCluster *infrav1.KubeforceCluster) error {
	buf := &bytes.Buffer{}
	externalAddress, err := r.getExternalAddresses(ctx, controlPlaneMachines)
	if err != nil {
		return err
	}
	if err := traefikConfigTmpl.Execute(buf, map[string]interface{}{
		"apiServers": externalAddress,
	}); err != nil {
		return err
	}
	cm.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(kubeforceCluster, infrav1.GroupVersion.WithKind("KubeforceCluster")),
	}
	cm.Labels = r.lbLabels(cluster.Name)
	cm.Data = map[string]string{
		"traefik.yaml": buf.String(),
	}
	return nil
}
func (r *KubeforceClusterReconciler) fillLBService(svc *corev1.Service, cluster *clusterv1.Cluster, kubeforceCluster *infrav1.KubeforceCluster) {
	svc.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(kubeforceCluster, infrav1.GroupVersion.WithKind("KubeforceCluster")),
	}
	svc.Labels = r.lbLabels(cluster.Name)
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	svc.Spec.Selector = r.lbLabels(cluster.Name)
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp",
			Protocol:   corev1.ProtocolTCP,
			Port:       9443,
			TargetPort: intstr.FromString("tcp"),
		},
	}
}

func (r *KubeforceClusterReconciler) fillLBDeployment(d *appsv1.Deployment,
	kubeforceCluster *infrav1.KubeforceCluster, cluster *clusterv1.Cluster) {
	d.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(kubeforceCluster, infrav1.GroupVersion.WithKind("KubeforceCluster")),
	}
	d.Labels = r.lbLabels(cluster.Name)
	d.Spec.Template.ObjectMeta.Labels = r.lbLabels(cluster.Name)
	d.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: r.lbLabels(cluster.Name),
	}
	if d.Spec.Template.Spec.Containers == nil {
		d.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
	}
	traefikContainer := d.Spec.Template.Spec.Containers[0]
	traefikContainer.Name = "traefik"
	traefikContainer.Image = "traefik:v2.9.6"
	traefikContainer.ImagePullPolicy = corev1.PullIfNotPresent
	traefikContainer.Args = []string{
		"--log.level=DEBUG",
		"--entrypoints.controlplane=true",
		"--entrypoints.controlplane.address=:9443",
		"--entrypoints.controlplane.transport.lifecycle.gracetimeout=10s",
		"--entrypoints.controlplane.transport.respondingtimeouts.readtimeout=10s",
		"--entrypoints.controlplane.transport.respondingtimeouts.idletimeout=10s",
		"--entrypoints.controlplane.transport.respondingtimeouts.writetimeout=10s",
		"--entrypoints.ping=true",
		"--entrypoints.ping.address=:9001",
		"--ping=true",
		"--ping.entrypoint=ping",
		"--providers.file.watch=true",
		"--providers.file.debugloggeneratedtemplate=true",
		"--providers.file.directory=/etc/traefik/dynamic_conf/",
	}
	traefikContainer.Ports = []corev1.ContainerPort{
		{
			Name:          "tcp",
			ContainerPort: 9443,
			Protocol:      "TCP",
		},
	}
	traefikContainer.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "config",
			ReadOnly:  true,
			MountPath: "/etc/traefik/dynamic_conf/",
		},
	}
	traefikContainer.LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/ping",
				Port:   intstr.FromInt(9001),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 2,
		TimeoutSeconds:      2,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	traefikContainer.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/ping",
				Port:   intstr.FromInt(9001),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 2,
		TimeoutSeconds:      2,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    1,
	}
	d.Spec.Template.Spec.Containers[0] = traefikContainer

	d.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: pointer.Int32(0o644),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.lbConfigName(cluster.Name),
					},
				},
			},
		},
	}
}

func (r *KubeforceClusterReconciler) lbLabels(clusterName string) map[string]string {
	return map[string]string{
		clusterv1.ClusterLabelName:    clusterName,
		"app.kubernetes.io/name":      "traefik",
		"app.kubernetes.io/component": "loadbalancer",
	}
}

func (r *KubeforceClusterReconciler) lbConfigName(clusterName string) string {
	return names.BuildName(clusterName, "-lb-config")
}

func (r *KubeforceClusterReconciler) reconcileControlPlaneEndpoint(ctx context.Context, cluster *clusterv1.Cluster,
	kubeforceCluster *infrav1.KubeforceCluster) error {
	patchObj := client.MergeFrom(kubeforceCluster.DeepCopy())
	apiserver := strings.Join([]string{cluster.Name, cluster.Namespace, "svc"}, ".")
	kubeforceCluster.Spec.ControlPlaneEndpoint.Host = apiserver
	kubeforceCluster.Spec.ControlPlaneEndpoint.Port = 9443

	if !cluster.Spec.ControlPlaneEndpoint.IsValid() {
		return nil
	}

	if cluster.Spec.ControlPlaneEndpoint.Host == kubeforceCluster.Spec.ControlPlaneEndpoint.Host &&
		cluster.Spec.ControlPlaneEndpoint.Port == kubeforceCluster.Spec.ControlPlaneEndpoint.Port {
		return nil
	}
	changed, err := patchutil.HasChanges(patchObj, kubeforceCluster)
	if err != nil {
		return errors.WithStack(err)
	}
	if changed {
		if err := r.Client.Patch(ctx, kubeforceCluster, patchObj); err != nil {
			return errors.Wrapf(err, "failed to patch KubeforceCluster")
		}
	}
	// delete ControlPlaneEndpoint in cluster
	clusterPatchData := client.MergeFrom(cluster.DeepCopy())
	cluster.Spec.ControlPlaneEndpoint.Port = 0
	cluster.Spec.ControlPlaneEndpoint.Host = ""
	if err := r.Client.Patch(ctx, cluster, clusterPatchData); err != nil {
		return errors.WithStack(err)
	}

	if err := r.deleteKubeconfig(ctx, cluster); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
func (r *KubeforceClusterReconciler) deleteKubeconfig(ctx context.Context, cluster *clusterv1.Cluster) error {
	clusterName := util.ObjectKey(cluster)
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WithStack(err)
	}
	if err == nil {
		if err := r.Client.Delete(ctx, configSecret); err != nil {
			return errors.WithStack(err)
		}
		ctrl.LoggerFrom(ctx).Info("kubeconfig has been deleted")
	}
	return nil
}

func (r *KubeforceClusterReconciler) refreshAPIServers(_ context.Context, cluster *infrav1.KubeforceCluster, controlPlaneMachines []infrav1.KubeforceMachine) {
	apiServers := make([]string, 0, len(controlPlaneMachines))
	for _, m := range controlPlaneMachines {
		if m.Status.DefaultIPAddress != "" {
			apiServers = append(apiServers, m.Status.DefaultIPAddress)
		}
	}
	sort.Strings(apiServers)
	cluster.Status.APIServers = apiServers
}

func (r *KubeforceClusterReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, kfCluster *infrav1.KubeforceCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	patchHelper, err := patch.NewHelper(kfCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(kfCluster, infrav1.InfrastructureAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchKubeforceCluster(ctx, patchHelper, kfCluster); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch KubeforceCluster")
	}

	descendants, err := r.listDescendants(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to list descendants")
		return reconcile.Result{}, err
	}

	if descendantCount := descendants.length(); descendantCount > 0 {
		log.Info("Cluster still has descendants - waiting for deletion", "descendants", descendants.descendantNames(), "count", descendants.length())
		// Requeue so we can check the next time to see if there are still any descendants left.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(kfCluster, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

// listDescendants returns a list of all MachineDeployments, MachineSets, MachinePools and Machines for the cluster.
func (r *KubeforceClusterReconciler) listDescendants(ctx context.Context, cluster *clusterv1.Cluster) (clusterDescendants, error) {
	var descendants clusterDescendants

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{clusterv1.ClusterLabelName: cluster.Name}),
	}

	if err := r.Client.List(ctx, &descendants.machines, listOptions...); err != nil {
		return descendants, errors.Wrapf(err, "failed to list KubeforceMachines for cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	if err := r.Client.List(ctx, &descendants.playbooks, listOptions...); err != nil {
		return descendants, errors.Wrapf(err, "failed to list Playbooks for cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	if err := r.Client.List(ctx, &descendants.playbookDeployments, listOptions...); err != nil {
		return descendants, errors.Wrapf(err, "failed to list PlaybookDeployments for cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	return descendants, nil
}

type clusterDescendants struct {
	machines            infrav1.KubeforceMachineList
	playbooks           infrav1.PlaybookList
	playbookDeployments infrav1.PlaybookDeploymentList
}

// length returns the number of descendants.
func (c *clusterDescendants) length() int {
	return len(c.machines.Items) +
		len(c.playbooks.Items) +
		len(c.playbookDeployments.Items)
}

func (c *clusterDescendants) descendantNames() string {
	descendants := make([]string, 0)
	kubeforceMachineNames := make([]string, len(c.machines.Items))
	for i, machine := range c.machines.Items {
		kubeforceMachineNames[i] = machine.Name
	}
	if len(kubeforceMachineNames) > 0 {
		descendants = append(descendants, "KubeforceMachines: "+strings.Join(kubeforceMachineNames, ","))
	}
	playbookNames := make([]string, len(c.playbooks.Items))
	for i, playbook := range c.playbooks.Items {
		playbookNames[i] = playbook.Name
	}
	if len(playbookNames) > 0 {
		descendants = append(descendants, "Playbooks: "+strings.Join(playbookNames, ","))
	}
	playbookDeploymentNames := make([]string, len(c.playbookDeployments.Items))
	for i, deployment := range c.playbookDeployments.Items {
		playbookDeploymentNames[i] = deployment.Name
	}
	if len(playbookDeploymentNames) > 0 {
		descendants = append(descendants, "Playbook deployments: "+strings.Join(playbookDeploymentNames, ","))
	}

	return strings.Join(descendants, ";")
}

func (r *KubeforceClusterReconciler) getControlPlaneMachineList(ctx context.Context, cluster *infrav1.KubeforceCluster) ([]infrav1.KubeforceMachine, error) {
	ml := &infrav1.KubeforceMachineList{}
	if err := r.Client.List(
		ctx,
		ml,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.MachineControlPlaneLabelName: "",
			clusterv1.ClusterLabelName:             cluster.Name,
		},
	); err != nil {
		return nil, errors.Wrap(err, "failed to list machines")
	}

	machines := make([]infrav1.KubeforceMachine, 0, len(ml.Items))
	for _, machine := range ml.Items {
		if machine.DeletionTimestamp.IsZero() {
			machines = append(machines, machine)
		}
	}

	return machines, nil
}

// KubeforceMachineToKubeforceCluster is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for KubeforceCluster based on updates to a KubeforceMachine.
func (r *KubeforceClusterReconciler) KubeforceMachineToKubeforceCluster(o client.Object) []ctrl.Request {
	m, ok := o.(*infrav1.KubeforceMachine)
	if !ok {
		r.Log.Info(fmt.Sprintf("Expected a KubeforceMachine but got a %T", o))
		return nil
	}
	_, isControlPlane := m.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]
	infraClusterName := m.Labels[infrav1.KubeforceClusterLabelName]
	if isControlPlane && infraClusterName != "" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: m.Namespace, Name: infraClusterName}}}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeforceClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	logger := ctrl.Log
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.KubeforceCluster{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(logger)).
		Build(r)

	if err != nil {
		return err
	}
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("KubeforceCluster"), mgr.GetClient(), &infrav1.KubeforceCluster{})),
		predicates.ClusterUnpaused(logger),
	)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &infrav1.KubeforceMachine{}},
		handler.EnqueueRequestsFromMapFunc(r.KubeforceMachineToKubeforceCluster),
		predicates.ResourceNotPaused(logger),
	)
}
