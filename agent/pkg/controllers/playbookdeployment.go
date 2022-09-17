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
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k3f.io/kubeforce/agent/pkg/util/checksum"
	"k3f.io/kubeforce/agent/pkg/util/conditions"
)

const (
	// PlaybookDeploymentFinalizer is the finalizer used by the controller to
	// cleanup the playbook resources.
	PlaybookDeploymentFinalizer = "playbookdeployment.agent.kubeforce.io"
)

var _ inject.Logger = &PlaybookDeploymentReconciler{}
var _ inject.Client = &PlaybookDeploymentReconciler{}

// PlaybookDeploymentReconciler reconciles a PlaybookDeployment object.
type PlaybookDeploymentReconciler struct {
	Client client.Client
	Log    logr.Logger
}

// InjectClient set client to the PlaybookDeploymentReconciler.
func (r *PlaybookDeploymentReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

// InjectLogger set logger to the PlaybookDeploymentReconciler.
func (r *PlaybookDeploymentReconciler) InjectLogger(log logr.Logger) error {
	r.Log = log.WithName("playbookDeployment")
	return nil
}

// Reconcile reconciles PlaybookDeployment.
func (r *PlaybookDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := r.Log.WithValues("request", req)
	log.Info("reconciling")
	pd := &v1alpha1.PlaybookDeployment{}
	err := r.Client.Get(context.Background(), req.NamespacedName, pd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// This object is automatically garbage collected.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Handle deletion reconciliation loop.
	if !pd.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, pd)
	}

	oldPb := pd.DeepCopy()
	if !controllerutil.ContainsFinalizer(pd, PlaybookDeploymentFinalizer) {
		controllerutil.AddFinalizer(pd, PlaybookDeploymentFinalizer)
		err := r.Client.Patch(ctx, pd, client.MergeFrom(oldPb))
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	defer func() {
		log.Info("reconcile completed")
		err := r.Client.Status().Patch(ctx, pd, client.MergeFrom(oldPb))
		if err != nil {
			log.Error(err, "unable to patch PlaybookDeployment")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()
	pd.Status.ObservedGeneration = pd.Generation
	if pd.Spec.Paused {
		pd.Status.Phase = v1alpha1.PlaybookDeploymentPaused
		return ctrl.Result{}, nil
	}

	lastPlaybook, err := r.getLastPlaybook(ctx, pd)
	if err != nil {
		return ctrl.Result{}, err
	}

	if lastPlaybook != nil &&
		!conditions.IsTrue(lastPlaybook, v1alpha1.PlaybookExecutionCondition) &&
		!conditions.IsTrue(lastPlaybook, v1alpha1.PlaybookFailedCondition) {
		pd.Status.Phase = v1alpha1.PlaybookDeploymentProgressing
		return ctrl.Result{}, nil
	}

	if lastPlaybook != nil {
		lastChecksum, err := checksum.CalcSHA256ForObject(&lastPlaybook.Spec)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		currentChecksum, err := checksum.CalcSHA256ForObject(&pd.Spec.Template.Spec)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		if lastChecksum == currentChecksum {
			pd.Status.Phase = r.getPlaybookDeploymentPhase(lastPlaybook)
			return ctrl.Result{}, nil
		}
	}

	// create a new playbook
	err = r.createPlaybook(ctx, pd)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	pd.Status.Phase = v1alpha1.PlaybookDeploymentProgressing
	return ctrl.Result{}, nil
}

func (r *PlaybookDeploymentReconciler) getLastPlaybook(ctx context.Context, pd *v1alpha1.PlaybookDeployment) (*v1alpha1.Playbook, error) {
	playbooks, err := r.getPlaybooksForDeployment(ctx, pd)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(playbooks) == 0 {
		return nil, nil
	}
	sortPlaybooksByCreationTime(playbooks)
	return playbooks[len(playbooks)-1], err
}

func sortPlaybooksByCreationTime(playbooks []*v1alpha1.Playbook) {
	sort.SliceStable(playbooks, func(i, j int) bool {
		return playbooks[i].CreationTimestamp.Before(&playbooks[j].CreationTimestamp)
	})
}

func (r *PlaybookDeploymentReconciler) getPlaybooksForDeployment(ctx context.Context, pd *v1alpha1.PlaybookDeployment) ([]*v1alpha1.Playbook, error) {
	list := &v1alpha1.PlaybookList{}
	err := r.Client.List(ctx, list)
	if err != nil {
		return nil, err
	}
	result := make([]*v1alpha1.Playbook, 0)
	for i := range list.Items {
		pl := &list.Items[i]
		if metav1.IsControlledBy(pl, pd) {
			result = append(result, pl)
		}
	}
	return result, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaybookDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PlaybookDeployment{}).
		Watches(
			&source.Kind{Type: &v1alpha1.Playbook{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &v1alpha1.PlaybookDeployment{},
				IsController: true,
			},
		).
		Complete(r)
}

func (r *PlaybookDeploymentReconciler) reconcileDelete(ctx context.Context, pd *v1alpha1.PlaybookDeployment) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(pd, PlaybookDeploymentFinalizer) {
		controllerutil.RemoveFinalizer(pd, PlaybookDeploymentFinalizer)
	}
	playbooks, err := r.getPlaybooksForDeployment(ctx, pd)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	for _, playbook := range playbooks {
		if playbook.DeletionTimestamp.IsZero() {
			if err := r.Client.Delete(ctx, playbook); err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *PlaybookDeploymentReconciler) createPlaybook(ctx context.Context, pd *v1alpha1.PlaybookDeployment) error {
	p := &v1alpha1.Playbook{
		ObjectMeta: metav1.ObjectMeta{
			Name:        names.SimpleNameGenerator.GenerateName(pd.Name + "-"),
			Labels:      pd.Spec.Template.Labels,
			Annotations: pd.Spec.Template.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "PlaybookDeployment",
					Name:       pd.Name,
					UID:        pd.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: pd.Spec.Template.Spec,
	}
	err := r.Client.Create(ctx, p)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (r *PlaybookDeploymentReconciler) getPlaybookDeploymentPhase(pl *v1alpha1.Playbook) v1alpha1.PlaybookDeploymentPhase {
	switch {
	case conditions.IsTrue(pl, v1alpha1.PlaybookExecutionCondition):
		return v1alpha1.PlaybookDeploymentSucceeded
	case conditions.IsTrue(pl, v1alpha1.PlaybookFailedCondition):
		return v1alpha1.PlaybookDeploymentFailed
	}
	return v1alpha1.PlaybookDeploymentProgressing
}
