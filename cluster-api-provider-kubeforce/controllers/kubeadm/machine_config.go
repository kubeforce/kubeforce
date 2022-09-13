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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Config = &machineConfig{}

type machineConfig struct {
	client  client.Client
	machine *clusterv1.Machine
}

func (m *machineConfig) IsDataAvailable() bool {
	return m.machine.Spec.Bootstrap.DataSecretName != nil
}

func (m *machineConfig) GetBootstrapData(ctx context.Context) ([]byte, error) {
	if m.machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: m.machine.GetNamespace(), Name: *m.machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(ctx, key, s); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for KubeforceMachine %s/%s", m.machine.GetNamespace(), m.machine.GetName())
	}

	value, ok := s.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}

func (m *machineConfig) GetKubeadmConfig(ctx context.Context) (*kubeadmv1.KubeadmConfig, error) {
	if m.machine.Spec.Bootstrap.ConfigRef == nil {
		return nil, errors.New("unable to get bootstrap config ref: linked Machine's bootstrap.configRef is nil")
	}
	if m.machine.Spec.Bootstrap.ConfigRef.Kind != "KubeadmConfig" {
		return nil, errors.Errorf("unknown type of bootstrap config: %v", m.machine.Spec.Bootstrap.ConfigRef.Kind)
	}
	cfg := &kubeadmv1.KubeadmConfig{}
	key := client.ObjectKey{Namespace: m.machine.Spec.Bootstrap.ConfigRef.Namespace, Name: m.machine.Spec.Bootstrap.ConfigRef.Name}
	if err := m.client.Get(ctx, key, cfg); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap config for Machine %s/%s", m.machine.GetNamespace(), m.machine.GetName())
	}
	return cfg, nil
}

func (m *machineConfig) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.machine)
}

func (m *machineConfig) GetKubernetesVersion() string {
	return pointer.StringDeref(m.machine.Spec.Version, "")
}
