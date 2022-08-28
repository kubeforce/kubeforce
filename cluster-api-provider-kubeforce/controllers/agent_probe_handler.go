package controllers

import (
	"context"
	"encoding/json"

	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/agent"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/controllers/prober"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAgentProbeHandler(key client.ObjectKey, client client.Client, agentClientCache *agent.ClientCache) prober.ProbeHandler {
	return &agentProbeHandler{
		key:              key,
		client:           client,
		agentClientCache: agentClientCache,
	}
}

type agentProbeHandler struct {
	key              client.ObjectKey
	client           client.Client
	agentClientCache *agent.ClientCache
}

func (h *agentProbeHandler) GetKey() string {
	return h.key.String()
}

func (h *agentProbeHandler) DoProbe(ctx context.Context) (bool, error) {
	clientset, err := h.agentClientCache.GetClientSet(ctx, h.key)
	if err != nil {
		return true, err
	}
	if clientset == nil {
		return false, nil
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
		conditions.MarkTrue(kfAgent, infrav1.Healthy)
	} else {
		conditions.MarkFalse(kfAgent, infrav1.Healthy, infrav1.ProbeFailedReason, clusterv1.ConditionSeverityInfo, result.Message)
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
