package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Helper provides reusable backup functionality for embedding in service maintenance types
type Helper struct {
	runner Runner
	log    logr.Logger
}

// NewHelper creates a new backup helper with the specified runner implementation
func NewHelper(runner Runner, log logr.Logger) *Helper {
	return &Helper{
		runner: runner,
		log:    log,
	}
}

// RunBackup executes a pre-maintenance backup using the configured runner
func (h *Helper) RunBackup(ctx context.Context) error {
	meta, err := GetBackupMetadata()
	if err != nil {
		return err
	}

	backupName := meta.FormatBackupName()
	h.log.Info("Running pre-maintenance backup",
		"namespace", meta.Namespace,
		"backupName", backupName)

	if err := h.runner.RunBackup(ctx, meta.Namespace, backupName); err != nil {
		return err
	}

	h.log.Info("Pre-maintenance backup completed successfully",
		"namespace", meta.Namespace,
		"backupName", backupName)

	return nil
}

// CheckDoneFunc checks if a resource has reached a terminal state
type CheckDoneFunc func(obj client.Object) bool

// CheckSuccessFunc checks if a completed resource succeeded or failed
type CheckSuccessFunc func(obj client.Object) error

// WatchUntilDone watches a Kubernetes resource until it reaches a terminal state
func WatchUntilDone(
	ctx context.Context,
	c client.WithWatch,
	obj client.Object,
	listObj client.ObjectList,
	timeout time.Duration,
	checkDone CheckDoneFunc,
	checkSuccess CheckSuccessFunc,
	log logr.Logger,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	targetName := obj.GetName()
	targetNamespace := obj.GetNamespace()

	// First, check if already complete
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return fmt.Errorf("failed to get initial resource state: %w", err)
	}

	if checkDone(obj) {
		log.Info("Resource already in terminal state")
		return checkSuccess(obj)
	}

	log.Info("Watching resource for completion",
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"namespace", targetNamespace,
		"name", targetName,
		"timeout", timeout)

	// Set up watch with field selector for the specific resource
	fieldSelector := fields.OneTermEqualSelector("metadata.name", targetName)
	watchOpts := &client.ListOptions{
		Namespace:     targetNamespace,
		FieldSelector: fieldSelector,
	}

	// Start the watch using the list object
	watcher, err := c.Watch(ctx, listObj, watchOpts)
	if err != nil {
		return fmt.Errorf("failed to create watch: %w", err)
	}

	// Define the condition function
	conditionFunc := func(event watch.Event) (bool, error) {
		eventObj, ok := event.Object.(client.Object)
		if !ok {
			log.V(1).Info("Received non-client.Object event", "type", fmt.Sprintf("%T", event.Object))
			return false, nil
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			if checkDone(eventObj) {
				log.Info("Resource reached terminal state",
					"kind", obj.GetObjectKind().GroupVersionKind().Kind,
					"namespace", targetNamespace,
					"name", targetName)
				return true, checkSuccess(eventObj)
			}
			log.V(1).Info("Resource updated but not yet complete")
			return false, nil

		case watch.Deleted:
			return false, fmt.Errorf("resource was deleted before completion")

		case watch.Error:
			if status, ok := event.Object.(*metav1.Status); ok {
				log.Info("Watch error event", "message", status.Message)
			}
			return false, nil

		default:
			return false, nil
		}
	}

	// Use UntilWithoutRetry for watching with the condition
	// We don't need the full retry machinery since we're watching a single resource
	_, err = watchtools.UntilWithoutRetry(ctx, watcher, conditionFunc)
	if err != nil {
		return fmt.Errorf("watch failed: %w", err)
	}

	return nil
}

// BaseRunner contains common fields shared by all backup runner implementations
type BaseRunner struct {
	k8sClient client.WithWatch
	log       logr.Logger
	timeout   time.Duration
}

// NewBaseRunner creates a new BaseRunner with default settings
func NewBaseRunner(c client.WithWatch, log logr.Logger) BaseRunner {
	return BaseRunner{
		k8sClient: c,
		log:       log,
		timeout:   1 * time.Hour,
	}
}

// TypeAssertObject performs type assertion with error logging
// Returns the typed object and a boolean indicating success
func TypeAssertObject[T client.Object](obj client.Object, expectedType string, log logr.Logger) (T, bool) {
	typed, ok := obj.(T)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type"), "Expected", expectedType, "got", fmt.Sprintf("%T", obj))
		var zero T
		return zero, false
	}
	return typed, true
}

// IsClusterSuspended checks if a cluster is suspended (instances == 0) and logs appropriately
func IsClusterSuspended(instances int, log logr.Logger, namespace string) bool {
	if instances == 0 {
		log.Info("Cluster is suspended (instances=0), skipping backup", "namespace", namespace)
		return true
	}
	return false
}
