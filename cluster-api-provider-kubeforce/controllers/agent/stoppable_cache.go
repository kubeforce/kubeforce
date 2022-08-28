package agent

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// stoppableCache embeds cache.Cache and combines it with a stop channel.
type stoppableCache struct {
	cache.Cache

	lock       sync.Mutex
	stopped    bool
	cancelFunc context.CancelFunc
}

// Stop cancels the cache.Cache's context, unless it has already been stopped.
func (cc *stoppableCache) Stop() {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.stopped {
		return
	}

	cc.stopped = true
	cc.cancelFunc()
}
