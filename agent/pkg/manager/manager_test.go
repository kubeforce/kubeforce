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
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type genRunnable func(chan string) []Runnable

func TestManager(t *testing.T) {
	tests := []struct {
		name                    string
		gracefulShutdownTimeout time.Duration
		genRunnables            genRunnable
		expectedMessages        []string
		wantErr                 bool
	}{
		{
			name:                    "correct sequence",
			gracefulShutdownTimeout: 10 * time.Second,
			genRunnables: func(msgCh chan string) []Runnable {
				return []Runnable{
					newReadyRunnable("main1", msgCh, 1*time.Second, 1*time.Second),
					newReadyRunnable("main2", msgCh, 2*time.Second, 2*time.Second),
					newRunnable("regular", msgCh, 0, 0),
				}
			},
			expectedMessages: []string{
				"start_main1",
				"start_main2",
				"start_regular",
				"end_regular",
				"end_main1",
				"end_main2",
			},
		},
		{
			name:                    "graceful shutdown timout - long main task",
			gracefulShutdownTimeout: 3 * time.Second,
			genRunnables: func(msgCh chan string) []Runnable {
				return []Runnable{
					newReadyRunnable("main", msgCh, 1*time.Second, 10*time.Second),
					newRunnable("regular", msgCh, 0, 0),
				}
			},
			expectedMessages: []string{
				"start_main",
				"start_regular",
				"end_regular",
			},
			wantErr: true,
		},
		{
			name:                    "graceful shutdown timout - long regular task",
			gracefulShutdownTimeout: 3 * time.Second,
			genRunnables: func(msgCh chan string) []Runnable {
				return []Runnable{
					newReadyRunnable("main", msgCh, 1*time.Second, 0),
					newRunnable("regular", msgCh, 0, 10*time.Second),
				}
			},
			expectedMessages: []string{
				"start_main",
				"start_regular",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mng, err := NewManager(tt.gracefulShutdownTimeout)
			if err != nil {
				t.Fatal(err)
			}
			msgCh := make(chan string)
			var messages []string
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					msg, ok := <-msgCh
					if !ok {
						return
					}
					messages = append(messages, msg)
				}
			}()
			runnables := tt.genRunnables(msgCh)
			for _, r := range runnables {
				mng.Add(r)
			}
			ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelFunc()

			if err := mng.Start(ctx); (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			close(msgCh)
			wg.Wait()
			if !cmp.Equal(messages, tt.expectedMessages) {
				t.Errorf("Manager.Start() diff= %v", cmp.Diff(messages, tt.expectedMessages))
			}
		})
	}
}

func newRunnable(name string, messageCh chan string, startDelay, endDelay time.Duration) Runnable {
	return &testRunnable{
		name:       name,
		messageCh:  messageCh,
		startDelay: startDelay,
		endDelay:   endDelay,
	}
}

type testRunnable struct {
	name       string
	messageCh  chan string
	startDelay time.Duration
	endDelay   time.Duration
}

func (r *testRunnable) Start(ctx context.Context) error {
	time.Sleep(r.startDelay)
	if !isClosed(r.messageCh) {
		r.messageCh <- "start_" + r.name
	}
	<-ctx.Done()
	time.Sleep(r.endDelay)
	if !isClosed(r.messageCh) {
		r.messageCh <- "end_" + r.name
	}
	return nil
}

func newReadyRunnable(name string, messageCh chan string, startDelay, endDelay time.Duration) ReadyRunnable {
	return &testReadyRunnable{
		name:       name,
		started:    make(chan struct{}),
		messageCh:  messageCh,
		startDelay: startDelay,
		endDelay:   endDelay,
	}
}

type testReadyRunnable struct {
	name       string
	started    chan struct{}
	messageCh  chan string
	startDelay time.Duration
	endDelay   time.Duration
}

func (r *testReadyRunnable) Start(ctx context.Context) error {
	time.Sleep(r.startDelay)
	if !isClosed(r.messageCh) {
		r.messageCh <- "start_" + r.name
	}
	close(r.started)
	<-ctx.Done()
	time.Sleep(r.endDelay)
	if !isClosed(r.messageCh) {
		r.messageCh <- "end_" + r.name
	}
	return nil
}

func (r *testReadyRunnable) ReadyNotify() <-chan struct{} {
	return r.started
}

func isClosed(ch <-chan string) bool {
	select {
	case <-ch:
		return true
	default:
	}

	return false
}
