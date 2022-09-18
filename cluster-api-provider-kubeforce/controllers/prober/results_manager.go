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

// NewResultItem creates a new ResultItem.
func NewResultItem(r ProbeResult, reason, message string) ResultItem {
	return ResultItem{
		ProbeResult: r,
		Time:        time.Now(),
		Reason:      reason,
		Message:     message,
	}
}

// ResultItem describe the result of probe result.
type ResultItem struct {
	ProbeResult
	Time    time.Time
	Reason  string
	Message string
}

// ToConditionStatus returns the ConditionStatus corresponding to this ResultItem.
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

// Get returns the cached ProbeResult for the container with the given ID.
func (m *resultManager) Get(key string) (ResultItem, bool) {
	m.RLock()
	defer m.RUnlock()
	ProbeResult, found := m.cache[key]
	return ProbeResult, found
}

// Set sets the cached ProbeResult for the container with the given ID.
func (m *resultManager) Set(key string, res ResultItem) {
	if m.setInternal(key, res) {
		m.updates <- Update{key, res}
	}
}

// Internal helper for locked portion of set. Returns whether an update should be sent.
func (m *resultManager) setInternal(key string, res ResultItem) bool {
	m.Lock()
	defer m.Unlock()
	updated := false
	prev, exists := m.cache[key]
	if !exists || prev.ProbeResult != res.ProbeResult {
		updated = true
	}
	m.cache[key] = res
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
