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

package controllers

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"k3f.io/kubeforce/agent/pkg/ansible"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k3f.io/kubeforce/agent/pkg/util/conditions"
)

const (
	// PlaybookFinalizer is the finalizer used by the controller to
	// cleanup the playbook resources.
	PlaybookFinalizer = "playbook.agent.kubeforce.io"
)

var (
	// DefaultJobBackOff is the default backoff period.
	DefaultJobBackOff = 10 * time.Second
	// MaxJobBackOff is the max backoff period.
	MaxJobBackOff = 360 * time.Second
)

var _ inject.Logger = &PlaybookReconciler{}
var _ inject.Client = &PlaybookReconciler{}

type PlaybookReconciler struct {
	PlaybookPath string
	Client       client.Client
	Log          logr.Logger
}

func (r *PlaybookReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *PlaybookReconciler) InjectLogger(log logr.Logger) error {
	r.Log = log.WithName("playbook")
	return nil
}

func (r *PlaybookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := r.Log.WithValues("request", req)
	log.Info("reconciling")
	pb := &v1alpha1.Playbook{}
	err := r.Client.Get(context.Background(), req.NamespacedName, pb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// This objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Handle deletion reconciliation loop.
	if !pb.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, pb)
	}

	oldPb := pb.DeepCopy()
	if !controllerutil.ContainsFinalizer(pb, PlaybookFinalizer) {
		controllerutil.AddFinalizer(pb, PlaybookFinalizer)
		err := r.Client.Patch(ctx, pb, client.MergeFrom(oldPb))
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	defer func() {
		log.Info("reconcile completed")
		err := r.Client.Status().Patch(ctx, pb, client.MergeFrom(oldPb))
		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if conditions.IsTrue(pb, v1alpha1.PlaybookExecutionCondition) || conditions.IsTrue(pb, v1alpha1.PlaybookFailedCondition) {
		return ctrl.Result{}, nil
	}
	if pb.Status.Failed >= *pb.Spec.Policy.BackoffLimit {
		pb.Status.Phase = v1alpha1.PlaybookFailed
		conditions.Set(pb, &v1alpha1.Condition{
			Type:    v1alpha1.PlaybookFailedCondition,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.BackoffLimitExceededReason,
			Message: "Playbook has reached the specified backoff limit",
		})
		return ctrl.Result{}, nil
	}
	if pb.Status.Failed > 0 {
		backoffTime := pb.ObjectMeta.CreationTimestamp.Add(getBackoff(pb.Status.Failed))
		now := time.Now()
		if now.Before(backoffTime) {
			return ctrl.Result{
				RequeueAfter: backoffTime.Sub(now),
			}, nil
		}
	}

	if pb.Status.Phase == "" || pb.Status.Phase == v1alpha1.PlaybookUnknown || pb.Status.Phase == v1alpha1.PlaybookFailed {
		pb.Status.Phase = v1alpha1.PlaybookPending
		return ctrl.Result{}, nil
	}

	if pb.Status.Phase == v1alpha1.PlaybookPending {
		helper := ansible.GetHelper()
		if err := helper.EnsureAnsible(ctx); err != nil {
			pb.Status.Phase = v1alpha1.PlaybookFailed
			pb.Status.Failed++
			conditions.MarkFalse(
				pb,
				v1alpha1.PlaybookExecutionCondition,
				v1alpha1.PlaybookPreparationFailedReason,
				err.Error())
		}
		pb.Status.Phase = v1alpha1.PlaybookRunning
		return ctrl.Result{}, nil
	}

	if pb.Status.Phase == v1alpha1.PlaybookRunning {
		err := r.runPlaybook(ctx, pb)
		if err != nil {
			pb.Status.Phase = v1alpha1.PlaybookFailed
			pb.Status.Failed++
			conditions.MarkFalse(
				pb,
				v1alpha1.PlaybookExecutionCondition,
				v1alpha1.PlaybookExecutionFailedReason,
				err.Error())
			r.Log.Error(err, "failed to execute the playbook", "req", req)
			return ctrl.Result{}, nil
		}
		pb.Status.Phase = v1alpha1.PlaybookSucceeded
		conditions.MarkTrue(pb, v1alpha1.PlaybookExecutionCondition)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaybookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Playbook{}).
		Complete(r)
}

func (r *PlaybookReconciler) reconcileDelete(ctx context.Context, pb *v1alpha1.Playbook) (ctrl.Result, error) {
	oldPb := pb.DeepCopy()
	dir := filepath.Join(r.PlaybookPath, pb.Name)
	err := os.RemoveAll(dir)
	if err != nil {
		r.Log.Error(err, "unable to remove root directory", "playbook", pb.Name, "dir", dir)
	}
	if controllerutil.ContainsFinalizer(pb, PlaybookFinalizer) {
		controllerutil.RemoveFinalizer(pb, PlaybookFinalizer)
		err := r.Client.Patch(ctx, pb, client.MergeFrom(oldPb))
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *PlaybookReconciler) runPlaybook(ctx context.Context, pb *v1alpha1.Playbook) error {
	err := os.MkdirAll(filepath.Join(r.PlaybookPath, pb.Name, "logs"), 0700)
	if err != nil {
		return err
	}
	for key, data := range pb.Spec.Files {
		filename := filepath.Join(r.PlaybookPath, pb.Name, key)
		err := os.MkdirAll(filepath.Dir(filename), 0700)
		if err != nil {
			return err
		}
		err = os.WriteFile(filename, []byte(data), 0600)
		if err != nil {
			return err
		}
	}
	ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{
		Connection: "local",
	}
	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Inventory: "127.0.0.1,",
	}
	now := time.Now()
	logFilename := now.Format("2006_01_02T15_04_05") + ".log"
	logFilePath := filepath.Join(r.PlaybookPath, pb.Name, "logs", logFilename)
	f, err := os.Create(logFilePath)
	if err != nil {
		return errors.Wrapf(err, "unable to create file %s", logFilePath)
	}
	defer f.Close()
	exec := execute.NewDefaultExecute(
		execute.WithWrite(io.Writer(f)),
		execute.WithWriteError(io.Writer(f)),
		execute.WithCmdRunDir(filepath.Join(r.PlaybookPath, pb.Name)),
	)

	cmd := &playbook.AnsiblePlaybookCmd{
		Playbooks:         []string{filepath.Join(r.PlaybookPath, pb.Name, pb.Spec.Entrypoint)},
		ConnectionOptions: ansiblePlaybookConnectionOptions,
		Options:           ansiblePlaybookOptions,
		Exec:              exec,
	}
	ctx, cancelFunc := context.WithTimeout(ctx, pb.Spec.Policy.Timeout.Duration)
	defer cancelFunc()
	err = cmd.Run(ctx)
	if err != nil {
		return err
	}
	return nil
}

func getBackoff(exp int32) time.Duration {
	if exp <= 0 {
		return time.Duration(0)
	}

	// The backoff is capped such that 'calculated' value never overflows.
	backoff := float64(DefaultJobBackOff.Nanoseconds()) * math.Pow(2, float64(exp-1))
	if backoff > math.MaxInt64 {
		return MaxJobBackOff
	}

	calculated := time.Duration(backoff)
	if calculated > MaxJobBackOff {
		return MaxJobBackOff
	}
	return calculated
}
