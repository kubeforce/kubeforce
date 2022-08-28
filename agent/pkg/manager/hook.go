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
