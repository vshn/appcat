package apiserver

import (
	"errors"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func ResolveError(groupResource schema.GroupResource, err error) error {
	statusErr := &apierrors.StatusError{}

	if errors.As(err, &statusErr) {
		switch {
		case apierrors.IsNotFound(err):
			return apierrors.NewNotFound(groupResource, statusErr.ErrStatus.Details.Name)
		case apierrors.IsAlreadyExists(err):
			return apierrors.NewAlreadyExists(groupResource, statusErr.ErrStatus.Details.Name)
		}
	}
	return err
}

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

func GetBackupTable(id, instance, status, age, started, finished string, backup runtime.Object) metav1.TableRow {
	return metav1.TableRow{
		Cells:  []interface{}{id, instance, started, finished, status, age}, // Snapshots are created only when the backup successfully finished
		Object: runtime.RawExtension{Object: backup},
	}
}

func GetBackupColumnDefinition() []metav1.TableColumnDefinition {
	desc := metav1.ObjectMeta{}.SwaggerDoc()
	return []metav1.TableColumnDefinition{
		{Name: "Backup ID", Type: "string", Format: "name", Description: desc["name"]},
		{Name: "Instance", Type: "string", Description: "The instance that this backup belongs to"},
		{Name: "Started", Type: "string", Description: "The backup start time"},
		{Name: "Finished", Type: "string", Description: "The data is available up to this time"},
		{Name: "Status", Type: "string", Description: "The state of this backup"},
		{Name: "Age", Type: "date", Description: desc["creationTimestamp"]},
	}
}

func IsTypeAvailable(gv string, kind string) bool {
	d, err := discovery.NewDiscoveryClientForConfig(loopback.GetLoopbackMasterClientConfig())
	if err != nil {
		return false
	}
	resources, err := d.ServerResourcesForGroupVersion(gv)
	if err != nil {
		return false
	}

	for _, res := range resources.APIResources {
		if res.Kind == kind {
			return true
		}
	}
	return false
}
