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
	"encoding/json"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	agentctrl "k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/prober"
)

// NewAgentProbeHandler creates a new ProbeHandler for the KubeforceAgent.
func NewAgentProbeHandler(key client.ObjectKey, client client.Client, agentClientCache *agentctrl.ClientCache) prober.ProbeHandler {
	return &agentProbeHandler{
		key:              key,
		client:           client,
		agentClientCache: agentClientCache,
	}
}

type agentProbeHandler struct {
	key              client.ObjectKey
	client           client.Client
	agentClientCache *agentctrl.ClientCache
}

func (h *agentProbeHandler) GetKey() string {
	return h.key.String()
}

func (h *agentProbeHandler) DoProbe(ctx context.Context) (bool, error) {
	clientset, err := h.agentClientCache.GetClientSet(ctx, h.key)
	if err != nil {
		return true, err
	}
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return true, err
	}
	return true, nil
}

func (h *agentProbeHandler) UpdateStatus(ctx context.Context, result prober.ResultItem) {
	kfAgent := &infrav1.KubeforceAgent{}
	if err := h.client.Get(ctx, h.key, kfAgent); err != nil {
		ctrl.LoggerFrom(ctx).
			WithValues("agent", h.key).
			Error(err, "unable to get agent")
		return
	}
	patchObj := client.MergeFrom(kfAgent.DeepCopy())
	if result.ProbeResult {
		conditions.MarkTrue(kfAgent, infrav1.HealthyCondition)
	} else {
		conditions.MarkFalse(kfAgent, infrav1.HealthyCondition, infrav1.ProbeFailedReason, clusterv1.ConditionSeverityInfo, result.Message)
	}

	diff, err := patchObj.Data(kfAgent)
	if err != nil {
		ctrl.LoggerFrom(ctx).
			WithValues("agent", h.key).
			Error(err, "failed to calculate patch data")
		return
	}

	// Unmarshal patch data into a local map.
	patchDiff := map[string]interface{}{}
	if err := json.Unmarshal(diff, &patchDiff); err != nil {
		ctrl.LoggerFrom(ctx).
			WithValues("agent", h.key).
			Error(err, "failed to unmarshal patch data into a map")
		return
	}

	if len(patchDiff) > 0 {
		if err := h.client.Status().Patch(ctx, kfAgent, patchObj); err != nil {
			ctrl.LoggerFrom(ctx).
				WithValues("agent", h.key).
				Error(err, "failed to patch KubeforceAgent")
			return
		}
	}
}
