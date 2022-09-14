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

package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Runnable allows a component to be started.
// It's very important that Start blocks until
// it's done running.
type Runnable interface {
	// Start starts running the component.  The component will stop running
	// when the context is closed. Start blocks until the context is closed or
	// an error occurs.
	Start(context.Context) error
}

// RunnableFunc implements Runnable using a function.
// It's very important that the given function block
// until it's done running.
type RunnableFunc func(context.Context) error

// Start implements Runnable.
func (r RunnableFunc) Start(ctx context.Context) error {
	return r(ctx)
}

// ReadyRunnable allows a component to be started.
// it sends signal when a component is ready
// These components will run first and finish last.
type ReadyRunnable interface {
	Runnable
	// ReadyNotify returns a channel that will be closed when the server
	// is ready to serve client requests
	ReadyNotify() <-chan struct{}
}

func NewManager(gracefulShutdownTimeout time.Duration) (*Manager, error) {
	return &Manager{
		runnables:               make([]Runnable, 0),
		gracefulShutdownTimeout: gracefulShutdownTimeout,
	}, nil
}

var _ Runnable = &Manager{}

type Manager struct {
	// mainRunnables are the components that run first and exit last.
	mainRunnables []ReadyRunnable
	// runnables is the regular components that run after main
	runnables []Runnable
	logger    logr.Logger
	// gracefulShutdownTimeout is the duration given to runnable to stop
	// before the manager actually returns on stop.
	gracefulShutdownTimeout time.Duration
	// waitForRunnable is holding the number of runnables currently running so that
	// we can wait for them to exit before quitting the manager
	waitForRunnable sync.WaitGroup
	// waitForMainRunnable is holding the number of mainRunnables currently running
	// we can wait for them to exit before quitting the manager
	waitForMainRunnable sync.WaitGroup
	errChan             chan error
}

func (m *Manager) Start(ctx context.Context) (err error) {
	mainCtx, mainCancelFunc := context.WithCancel(context.Background())
	defer mainCancelFunc()
	internalCtx, internalCancel := context.WithCancel(ctx)
	defer func() {
		internalCancel()
		stopErr := m.waitForRunnableToEnd()
		if stopErr != nil {
			if err != nil {
				err = kerrors.NewAggregate([]error{err, stopErr})
			} else {
				err = stopErr
			}
		}
	}()

	m.errChan = make(chan error)

	for _, r := range m.mainRunnables {
		// Controllers block, but we want to return an error if any have an error starting.
		// Write any Start errors to a channel so we can return them
		m.startRunnable(mainCtx, r, &m.waitForMainRunnable)
	}

	// wait to start main components
	for _, r := range m.mainRunnables {
		select {
		case <-r.ReadyNotify():
			continue
		case err := <-m.errChan:
			// Error starting or running a runnable
			return err
		}
	}

	for _, r := range m.runnables {
		// Controllers block, but we want to return an error if any have an error starting.
		// Write any Start errors to a channel so we can return them
		m.startRunnable(internalCtx, r, &m.waitForRunnable)
	}

	go func() {
		m.waitForRunnable.Wait()
		mainCancelFunc()
	}()

	select {
	case <-ctx.Done():
		// We are done
		return nil
	case err := <-m.errChan:
		// Error starting or running a runnable
		return err
	}
}

func (m *Manager) startRunnable(ctx context.Context, r Runnable, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.Start(ctx); err != nil {
			m.errChan <- err
		}
	}()
}

// waitForRunnableToEnd blocks until all runnables ended or the
// tearDownTimeout was reached. In the latter case, an error is returned.
func (m *Manager) waitForRunnableToEnd() (retErr error) {
	var shutdownCtx context.Context
	var shutdownCancel context.CancelFunc
	if m.gracefulShutdownTimeout > 0 {
		shutdownCtx, shutdownCancel = context.WithTimeout(context.Background(), m.gracefulShutdownTimeout)
	} else {
		shutdownCtx, shutdownCancel = context.WithCancel(context.Background())
	}

	go func() {
		for {
			select {
			case err, ok := <-m.errChan:
				if ok {
					m.logger.Error(err, "error received after stop sequence was engaged")
				}
			case <-shutdownCtx.Done():
				return
			}
		}
	}()
	go func() {
		m.waitForRunnable.Wait()
		m.waitForMainRunnable.Wait()
		shutdownCancel()
	}()

	<-shutdownCtx.Done()
	if err := shutdownCtx.Err(); err != nil && err != context.Canceled {
		return fmt.Errorf("failed waiting for all runnables to end within grace period of %s: %w", m.gracefulShutdownTimeout, err)
	}
	return nil
}

func (m *Manager) Add(r Runnable) {
	if rr, ok := r.(ReadyRunnable); ok {
		m.mainRunnables = append(m.mainRunnables, rr)
	} else {
		m.runnables = append(m.runnables, r)
	}
}
