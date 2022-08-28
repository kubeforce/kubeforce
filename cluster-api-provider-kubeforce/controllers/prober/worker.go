package prober

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// worker handles the periodic probing of its assigned container. Each worker has a go-routine
// associated with it which runs the probe loop until the container permanently terminates, or the
// stop channel is closed. The worker uses the probe Manager's statusManager to get up-to-date
// container IDs.
type worker struct {
	// Channel for stopping the probe.
	stopCh   chan struct{}
	stopOnce sync.Once

	// Lock for accessing & mutating probeParams
	probeParamsLock sync.RWMutex
	probeParams     ProbeParams

	// Channel for waiting the probe.
	waitCh chan struct{}

	probe ProbeHandler

	probeController *controller

	// The last probe result for this worker.
	lastResult ProbeResult
	// How many times in a row the probe has returned the same result.
	resultRun int

	// proberResultsMetricLabels holds the labels attached to this worker
	// for the ProberResults metric by result.
	proberResultsSuccessfulMetricLabels prometheus.Labels
	proberResultsFailedMetricLabels     prometheus.Labels
}

const (
	probeResultSuccessful string = "successful"
	probeResultFailed     string = "failed"
)

// Creates and starts a new probe worker.
func newWorker(m *controller, probe ProbeHandler, probeParams ProbeParams) *worker {

	w := &worker{
		stopCh:          make(chan struct{}),
		waitCh:          make(chan struct{}),
		probe:           probe,
		probeParams:     probeParams,
		probeController: m,
	}

	basicMetricLabels := prometheus.Labels{
		"probe_key": probe.GetKey(),
	}

	w.proberResultsSuccessfulMetricLabels = deepCopyPrometheusLabels(basicMetricLabels)
	w.proberResultsSuccessfulMetricLabels["result"] = probeResultSuccessful

	w.proberResultsFailedMetricLabels = deepCopyPrometheusLabels(basicMetricLabels)
	w.proberResultsFailedMetricLabels["result"] = probeResultFailed

	return w
}

// run periodically probes the container.
func (w *worker) run(ctx context.Context) {

	// If controller restarted the probes could be started in rapid succession.
	// Let the worker wait for a random portion of tickerPeriod before probing.
	time.Sleep(time.Duration(rand.Float64() * float64(time.Duration(w.getProbeParams().PeriodSeconds)*time.Second)))

	defer func() {
		// Clean up.
		ProberResults.Delete(w.proberResultsSuccessfulMetricLabels)
		ProberResults.Delete(w.proberResultsFailedMetricLabels)
		// close waitCh before remove worker from prob controller
		w.stop()
		w.probeController.removeWorker(w.probe.GetKey())
	}()

probeLoop:
	for w.doProbe(ctx) {
		// Wait for next probe tick.
		select {
		case <-ctx.Done():
			break probeLoop
		case <-w.stopCh:
			break probeLoop
		case <-time.After(time.Duration(w.getProbeParams().PeriodSeconds) * time.Second):
			// continue
		}
	}
}

// stop the probe worker. The worker handles cleanup and removes itself from its controller.
// It is safe to call stop multiple times.
func (w *worker) stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

// wait until worker is working
func (w *worker) wait(timeout time.Duration) error {
	waitTimeout := time.NewTimer(timeout)
	select {
	// block until waitCh is not closed
	case <-w.waitCh:
		waitTimeout.Stop()
		return nil
	case <-waitTimeout.C:
		waitTimeout.Stop()
		return errors.New("timeout")
	}
}

func (w *worker) getProbeParams() ProbeParams {
	w.probeParamsLock.RLock()
	params := w.probeParams
	w.probeParamsLock.RUnlock()
	return params
}

func (w *worker) setProbeParams(p ProbeParams) {
	w.probeParamsLock.Lock()
	w.probeParams = p
	w.probeParamsLock.Unlock()
}

// doProbe probes the container once and records the result.
// Returns whether the worker should continue.
func (w *worker) doProbe(ctx context.Context) (keepGoing bool) {
	defer runtime.HandleCrash(func(_ interface{}) { keepGoing = true })

	// get params read-only
	params := w.getProbeParams()

	reqContext, cancelFn := context.WithTimeout(ctx, time.Duration(params.TimeoutSeconds)*time.Second)
	defer cancelFn()
	keepGoing, err := w.probe.DoProbe(reqContext)
	result := ProbeResult(err == nil)

	if result {
		ProberResults.With(w.proberResultsSuccessfulMetricLabels).Inc()
	} else {
		ProberResults.With(w.proberResultsFailedMetricLabels).Inc()
	}
	if w.lastResult == result {
		w.resultRun++
	} else {
		w.lastResult = result
		w.resultRun = 1
	}

	if (result == ResultFailure && w.resultRun < int(params.FailureThreshold)) ||
		(result == ResultSuccess && w.resultRun < int(params.SuccessThreshold)) {
		// Success or failure is below threshold - leave the probe state unchanged.
		return keepGoing
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	w.probeController.resultManager.Set(w.probe.GetKey(), NewResultItem(result, result.String(), msg))
	return keepGoing
}

func deepCopyPrometheusLabels(m prometheus.Labels) prometheus.Labels {
	ret := make(prometheus.Labels, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
