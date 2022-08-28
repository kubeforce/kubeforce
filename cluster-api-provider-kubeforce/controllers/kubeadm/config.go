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
