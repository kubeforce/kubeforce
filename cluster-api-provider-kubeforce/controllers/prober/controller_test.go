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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func init() {
	runtime.ReallyCrash = true
}

type fakeProbeHandler struct {
	key         string
	resultMu    sync.RWMutex
	probeResult error
	updateMu    sync.RWMutex
	update      *ResultItem
}

func (f *fakeProbeHandler) GetKey() string {
	return f.key
}

func (f *fakeProbeHandler) DoProbe(ctx context.Context) (bool, error) {
	f.resultMu.RLock()
	defer f.resultMu.RUnlock()
	return true, f.probeResult
}

func (f *fakeProbeHandler) UpdateStatus(ctx context.Context, result ResultItem) {
	f.updateMu.Lock()
	defer f.updateMu.Unlock()
	f.update = &result
}

func (f *fakeProbeHandler) setFakeProbeResult(err error) {
	f.resultMu.Lock()
	defer f.resultMu.Unlock()
	f.probeResult = err
}

func (f *fakeProbeHandler) getUpdateStatus() *ResultItem {
	f.updateMu.RLock()
	defer f.updateMu.RUnlock()
	return f.update
}

func newFakeProbeHandler(key string, probeResult error) *fakeProbeHandler {
	return &fakeProbeHandler{
		key:         key,
		probeResult: probeResult,
	}
}

var _ ProbeHandler = &fakeProbeHandler{}

func testProbeParams() ProbeParams {
	return ProbeParams{
		TimeoutSeconds:   1,
		PeriodSeconds:    1,
		SuccessThreshold: 2,
		FailureThreshold: 2,
	}
}

func TestAddRemoveProbes(t *testing.T) {
	ctx := context.Background()

	successProbe := newFakeProbeHandler("success-test", nil)
	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	c := NewController(logger).(*controller)
	defer cleanup(t, c)
	if err := expectProbes(c, nil); err != nil {
		t.Error(err)
	}

	c.EnsureProbe(ctx, successProbe, testProbeParams())
	probePaths := []string{"success-test"}
	if err := expectProbes(c, probePaths); err != nil {
		t.Error(err)
	}

	// Removing non-existent.
	c.RemoveProbe("empty")
	if err := expectProbes(c, probePaths); err != nil {
		t.Error(err)
	}

	c.RemoveProbe("success-test")
	if err := waitForWorkerExit(t, c, probePaths); err != nil {
		t.Fatal(err)
	}
	if err := expectProbes(c, nil); err != nil {
		t.Error(err)
	}

	c.RemoveProbe("success-test")
	if err := expectProbes(c, nil); err != nil {
		t.Error(err)
	}
}

func TestUpdate(t *testing.T) {
	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	c := NewController(logger).(*controller)
	defer cleanup(t, c)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	go func() {
		_ = c.Start(ctx)
	}()

	probeHandler := newFakeProbeHandler("success-test", nil)
	if updStatus := probeHandler.getUpdateStatus(); updStatus != nil {
		t.Errorf("UpdateStatus status is not nil %v: ", updStatus)
	}
	c.EnsureProbe(ctx, probeHandler, testProbeParams())

	probePaths := []string{"success-test"}
	if err := expectProbes(c, probePaths); err != nil {
		t.Error(err)
	}

	// Wait for ready status.
	if err := waitForReadyStatus(t, c, "success-test", metav1.ConditionTrue); err != nil {
		t.Error(err)
	}
	if updStatus := probeHandler.getUpdateStatus(); updStatus == nil || updStatus.ProbeResult != ResultSuccess {
		t.Errorf("Unexpected status of update for probe %v: Expected %v but got %v",
			probeHandler.key, ResultSuccess, updStatus)
	}
	errMsg := "custom error message"
	probeHandler.setFakeProbeResult(errors.New(errMsg))

	// Wait for failed status.
	if err := waitForReadyStatus(t, c, "success-test", metav1.ConditionFalse); err != nil {
		t.Error(err)
	}
	if updStatus := probeHandler.getUpdateStatus(); updStatus == nil || updStatus.ProbeResult != ResultFailure || updStatus.Message != errMsg {
		t.Errorf("Unexpected status of update for probe %v: Expected %v but got %v",
			probeHandler.key, ResultFailure, updStatus)
	}
}

func expectProbes(m *controller, expectedProbes []string) error {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	var unexpected []string
	missing := make([]string, len(expectedProbes))
	copy(missing, expectedProbes)

outer:
	for probePath := range m.workers {
		for i, expectedPath := range missing {
			if probePath == expectedPath {
				missing = append(missing[:i], missing[i+1:]...)
				continue outer
			}
		}
		unexpected = append(unexpected, probePath)
	}

	if len(missing) == 0 && len(unexpected) == 0 {
		return nil // Yay!
	}

	return fmt.Errorf("unexpected probes: %v, missing probes: %v", unexpected, missing)
}

const interval = 1 * time.Second

// Wait for the given workers to exit & clean up.
func waitForWorkerExit(t *testing.T, m *controller, workerKeys []string) error {
	t.Helper()
	for _, key := range workerKeys {
		condition := func() (bool, error) {
			_, exists := m.getWorker(key)
			return !exists, nil
		}
		if exited, _ := condition(); exited {
			continue // Already exited, no need to poll.
		}
		t.Logf("Polling %v", key)
		if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
			return err
		}
	}

	return nil
}

// Wait for the given workers to exit & clean up.
func waitForReadyStatus(t *testing.T, m *controller, key string, readyStatus metav1.ConditionStatus) error {
	t.Helper()
	condition := func() (bool, error) {
		result, ok := m.resultManager.Get(key)
		return ok && result.ToConditionStatus() == readyStatus, nil
	}
	t.Logf("Polling for ready state %v", readyStatus)
	if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
		return err
	}

	return nil
}

func (m *controller) cleanup() {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	for _, worker := range m.workers {
		worker.stop()
	}
}

// workerCount returns the total number of probe workers. For testing.
func (m *controller) workerCount() int {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	return len(m.workers)
}

func (m *controller) getWorker(key string) (*worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	worker, ok := m.workers[key]
	return worker, ok
}

// cleanup running probes to avoid leaking goroutines.
func cleanup(t *testing.T, m *controller) {
	t.Helper()
	m.cleanup()

	condition := func() (bool, error) {
		workerCount := m.workerCount()
		if workerCount > 0 {
			t.Logf("Waiting for %d workers to exit...", workerCount)
		}
		return workerCount == 0, nil
	}
	if exited, _ := condition(); exited {
		return // Already exited, no need to poll.
	}
	if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
		t.Fatalf("Error during cleanup: %v", err)
	}
}
