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
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util"
	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	utilexp "sigs.k8s.io/cluster-api/exp/util"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config interface {
	IsDataAvailable() bool
	GetBootstrapData(ctx context.Context) ([]byte, error)
	GetKubeadmConfig(ctx context.Context) (*kubeadmv1.KubeadmConfig, error)
	// IsControlPlane returns true if this machine is a control plane node.
	IsControlPlane() bool
	// GetKubernetesVersion returns version of kubernetes for this machine
	GetKubernetesVersion() string
}

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
	kubeforceMachinePool, err := util.GetOwnerKubeforceMachinePool(ctx, client, kubeforceMachine.ObjectMeta)
	if kubeforceMachinePool != nil {
		machinePool, err := utilexp.GetOwnerMachinePool(ctx, client, kubeforceMachinePool.ObjectMeta)
		if err != nil {
			return nil, errors.WithMessage(err, "unable to get owner MachinePool for KubeforceMachinePool")
		}
		if machinePool != nil {
			return &machinePoolConfig{
				client:      client,
				machinePool: machinePool,
			}, nil
		}
	}
	return nil, nil
}
