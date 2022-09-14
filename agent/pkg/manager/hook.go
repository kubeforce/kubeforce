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

	genericapiserver "k8s.io/apiserver/pkg/server"
)

func NewKubeHook(r Runnable) *KubeHook {
	return &KubeHook{
		r:       r,
		stopped: make(chan struct{}),
	}
}

type KubeHook struct {
	r          Runnable
	cancelFunc context.CancelFunc
	stopped    chan struct{}
}

func (h *KubeHook) PostStartHookFunc() genericapiserver.PostStartHookFunc {
	return func(hookCtx genericapiserver.PostStartHookContext) error {
		ctx, cancelFunc := context.WithCancel(context.Background())
		h.cancelFunc = cancelFunc
		defer cancelFunc()
		defer func() {
			close(h.stopped)
		}()
		go func() {
			<-hookCtx.StopCh
			cancelFunc()
		}()
		if err := h.r.Start(ctx); err != nil {
			return err
		}
		return nil
	}
}

func (h *KubeHook) PreShutdownHookFunc() genericapiserver.PreShutdownHookFunc {
	return func() error {
		<-h.stopped
		return nil
	}
}
