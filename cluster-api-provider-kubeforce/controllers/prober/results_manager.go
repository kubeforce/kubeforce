package prober

import (
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResultManager provides a probe results cache and channel of updates.
type ResultManager interface {
	// Get returns the cached ProbeResult for the container with the given ID.
	Get(string) (ResultItem, bool)
	// Set sets the cached ProbeResult for the container with the given ID.
	Set(string, ResultItem)
	// Remove clears the cached ProbeResult for the container with the given ID.
	Remove(string)
	// Updates creates a channel that receives an Update whenever its ProbeResult changes (but not
	// removed).
	// NOTE: The current implementation only supports a single updates channel.
	Updates() <-chan Update
}

func NewResultItem(r ProbeResult, reason, message string) ResultItem {
	return ResultItem{
		ProbeResult: r,
		Time:        time.Now(),
		Reason:      reason,
		Message:     message,
	}
}

type ResultItem struct {
	ProbeResult
	Time    time.Time
	Reason  string
	Message string
}

func (i ResultItem) ToConditionStatus() metav1.ConditionStatus {
	if i.ProbeResult {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func (r ProbeResult) String() string {
	switch r {
	case ResultSuccess:
		return "Success"
	case ResultFailure:
		return "Failure"
	default:
		return "UNKNOWN"
	}
}

// Update is an enum of the types of updates sent over the Updates channel.
type Update struct {
	Key    string
	Result ResultItem
}

// Manager implementation.
type resultManager struct {
	// guards the cache
	sync.RWMutex
	// map of container ID -> probe ProbeResult
	cache map[string]ResultItem
	// channel of updates
	updates chan Update
}

var _ ResultManager = &resultManager{}

// NewResultManager creates and returns an empty resultManager.
func NewResultManager() ResultManager {
	return &resultManager{
		cache:   make(map[string]ResultItem),
		updates: make(chan Update, 20),
	}
}

func (m *resultManager) Get(key string) (ResultItem, bool) {
	m.RLock()
	defer m.RUnlock()
	ProbeResult, found := m.cache[key]
	return ProbeResult, found
}

func (m *resultManager) Set(key string, ProbeResult ResultItem) {
	if m.setInternal(key, ProbeResult) {
		m.updates <- Update{key, ProbeResult}
	}
}

// Internal helper for locked portion of set. Returns whether an update should be sent.
func (m *resultManager) setInternal(key string, ProbeResult ResultItem) bool {
	m.Lock()
	defer m.Unlock()
	updated := false
	prev, exists := m.cache[key]
	if !exists || prev.ProbeResult != ProbeResult.ProbeResult {
		updated = true
	}
	m.cache[key] = ProbeResult
	return updated
}

func (m *resultManager) Remove(key string) {
	m.Lock()
	defer m.Unlock()
	delete(m.cache, key)
}

func (m *resultManager) Updates() <-chan Update {
	return m.updates
}
