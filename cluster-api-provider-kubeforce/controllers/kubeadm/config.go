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

package kubeadm

import (
	"context"

	"github.com/pkg/errors"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
)

// Config is an interface for kubeadm config.
type Config interface {
	IsDataAvailable() bool
	GetBootstrapData(ctx context.Context) ([]byte, error)
	GetKubeadmConfig(ctx context.Context) (*bootstrapv1.KubeadmConfig, error)
	// IsControlPlane returns true if this machine is a control plane node.
	IsControlPlane() bool
	// GetKubernetesVersion returns version of kubernetes for this machine
	GetKubernetesVersion() string
}

// GetConfig returns a kubeadm Config for KubeforceMachine.
func GetConfig(ctx context.Context, client client.Client, kubeforceMachine *infrav1.KubeforceMachine) (Config, error) {
	machine, err := capiutil.GetOwnerMachine(ctx, client, kubeforceMachine.ObjectMeta)
	if err != nil {
		return nil, errors.WithMessage(err, "unable to get owner machine for KubeforceMachine")
	}
	if machine != nil {
		return &machineConfig{
			client:  client,
			machine: machine,
		}, nil
	}
	return nil, nil
}
