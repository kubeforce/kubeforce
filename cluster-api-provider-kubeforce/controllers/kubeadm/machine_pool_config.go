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
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Config = &machinePoolConfig{}

type machinePoolConfig struct {
	client      client.Client
	machinePool *expv1.MachinePool
}

func (m *machinePoolConfig) IsDataAvailable() bool {
	return m.machinePool.Spec.Template.Spec.Bootstrap.DataSecretName != nil
}

func (m *machinePoolConfig) GetBootstrapData(ctx context.Context) ([]byte, error) {
	if m.machinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked bootstrap.dataSecretName is nil")
	}

	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: m.machinePool.GetNamespace(), Name: *m.machinePool.Spec.Template.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(ctx, key, s); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for KubeforceMachine %s/%s", m.machinePool.GetNamespace(), m.machinePool.GetName())
	}

	value, ok := s.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}

func (m *machinePoolConfig) GetKubeadmConfig(ctx context.Context) (*bootstrapv1.KubeadmConfig, error) {
	if m.machinePool.Spec.Template.Spec.Bootstrap.ConfigRef == nil {
		return nil, errors.New("unable to get bootstrap config ref: linked Machine's bootstrap.configRef is nil")
	}
	if m.machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Kind != "KubeadmConfig" {
		return nil, errors.Errorf("unknown type of bootstrap config: %v", m.machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Kind)
	}
	cfg := &bootstrapv1.KubeadmConfig{}
	key := client.ObjectKey{Namespace: m.machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Namespace, Name: m.machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Name}
	if err := m.client.Get(ctx, key, cfg); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap config for Machine %s/%s", m.machinePool.GetNamespace(), m.machinePool.GetName())
	}
	return cfg, nil
}

func (m *machinePoolConfig) IsControlPlane() bool {
	return false
}

func (m *machinePoolConfig) GetKubernetesVersion() string {
	return pointer.StringDeref(m.machinePool.Spec.Template.Spec.Version, "")
}
