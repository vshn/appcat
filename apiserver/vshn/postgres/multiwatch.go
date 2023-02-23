package postgres

import (
	"k8s.io/apimachinery/pkg/watch"
	"sync"
)

// MultiWatch is wrapper of multiple source watches which implements the same methods as a normal watch.Watch
var _ watch.Interface = &MultiWatcher{}

type MultiWatcher struct {
	watchers  []watch.Interface
	eventChan chan watch.Event
	wg        sync.WaitGroup
}

// NewEmptyMultiWatch creates an empty watch
func NewEmptyMultiWatch() *MultiWatcher {
	return &MultiWatcher{
		eventChan: make(chan watch.Event),
	}
}

// AddWatcher adds a watch to this MultiWatcher
func (m *MultiWatcher) AddWatcher(w watch.Interface) {
	m.watchers = append(m.watchers, w)
}

// Stop stops all watches of this MultiWatch
func (m *MultiWatcher) Stop() {
	for _, watcher := range m.watchers {
		watcher.Stop()
	}
	m.wg.Wait()
	close(m.eventChan)
}

// ResultChan aggregates all channels from all watches of this MultiWatch
func (m *MultiWatcher) ResultChan() <-chan watch.Event {
	for _, w := range m.watchers {
		m.wg.Add(1)
		watcher := w
		go func() {
			defer m.wg.Done()
			for c := range watcher.ResultChan() {
				m.eventChan <- c
			}
		}()
	}
	return m.eventChan
}
