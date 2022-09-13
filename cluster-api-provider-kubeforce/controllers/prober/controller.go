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

package prober

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/component-base/metrics"
)

// ProbeResult is the type for probe results.
type ProbeResult bool

const (
	// ResultSuccess is encoded as "true" (type Result)
	ResultSuccess ProbeResult = true

	// ResultFailure is encoded as "false" (type Result)
	ResultFailure ProbeResult = false
)

type ProbeHandler interface {
	GetKey() string
	DoProbe(ctx context.Context) (bool, error)
	UpdateStatus(ctx context.Context, result ResultItem)
}

// ProberResults stores the cumulative number of a probe by result as prometheus metrics.
var ProberResults = metrics.NewCounterVec(
	&metrics.CounterOpts{
		Subsystem:      "prober",
		Name:           "probe_total",
		Help:           "Cumulative number of a metrics and readiness probe for a server by result.",
		StabilityLevel: metrics.ALPHA,
	},
	[]string{"probe_key",
		"result"},
)

// ProbeParams describes a health check params
type ProbeParams struct {
	// Number of seconds after which the probe times out.
	// Defaults to 3 second. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 5 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 2. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

func (p ProbeParams) Equal(o ProbeParams) bool {
	return p.TimeoutSeconds == o.TimeoutSeconds &&
		p.PeriodSeconds == o.PeriodSeconds &&
		p.SuccessThreshold == o.SuccessThreshold &&
		p.FailureThreshold == o.FailureThreshold
}

// Controller controls probing. It creates a probe "worker" for every probe.
//  The worker periodically probes its assigned validation method and caches the results. The
// controller use the cached probe results to set the appropriate Ready state in the condition.
type Controller interface {
	// EnsureProbe creates new probe workers if necessary.
	EnsureProbe(ctx context.Context, probe ProbeHandler, params ProbeParams)

	// RemoveProbe handles cleaning up, terminating and removing probe workers.
	RemoveProbe(key string)

	// GetCurrentStatus returns current status of probe
	// returns empty if there is no probe
	GetCurrentStatus(key string) *ResultItem

	// Start starts the Manager sync loops.
	Start(ctx context.Context) error
}

// NewController creates a Controller for probing.
func NewController(log logr.Logger) Controller {
	return &controller{
		log:           log,
		workers:       make(map[string]*worker),
		resultManager: NewResultManager(),
	}
}

type controller struct {
	// log is a logger for the probe controller
	log logr.Logger
	// Map of active workers for probes
	workers map[string]*worker
	// Lock for accessing & mutating workers
	workerLock sync.RWMutex
	// resultManager manages the results of probes
	resultManager ResultManager
}

func (m *controller) EnsureProbe(ctx context.Context, probe ProbeHandler, params ProbeParams) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	key := probe.GetKey()
	if oldWorker, ok := m.workers[key]; ok {
		oldParams := oldWorker.getProbeParams()
		if !oldParams.Equal(params) {
			m.log.Info("the probe has been changed params", "key", key, "oldParams", oldParams, "newParams", params)
			oldWorker.setProbeParams(params)
		}
		return
	}
	w := newWorker(m, probe, params)
	m.workers[key] = w
	go w.run(ctx)
}

func (m *controller) RemoveProbe(key string) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	if worker, ok := m.workers[key]; ok {
		worker.stop()
	}
}

// Start syncing probe status. This should only be called once.
func (m *controller) Start(ctx context.Context) error {
	// Start syncing readiness.
	wait.UntilWithContext(ctx, m.updateStatus, 0)
	return nil
}

func (m *controller) GetCurrentStatus(key string) *ResultItem {
	result, found := m.resultManager.Get(key)
	if !found {
		return nil
	}
	return &result
}

func (m *controller) updateStatus(ctx context.Context) {
	update := <-m.resultManager.Updates()
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	if worker, ok := m.workers[update.Key]; ok {
		worker.probe.UpdateStatus(ctx, update.Result)
	}
}

// Called by the worker after exiting.
func (m *controller) removeWorker(key string) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, key)
}
