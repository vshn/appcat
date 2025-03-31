package k8up

import (
	"context"
	"fmt"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="k8up.io",resources=snapshots,verbs=get;list;watch

var (
	K8upGVR = schema.GroupVersionResource{
		Group:    k8upv1.GroupVersion.Group,
		Version:  k8upv1.GroupVersion.Version,
		Resource: "snapshots",
	}
)

var _ Snapshothandler = &ConcreteSnapshotHandler{}

// Snapshothandler is an interface that handles listing and getting of k8up snapsthots.
type Snapshothandler interface {
	Get(ctx context.Context, id, instanceNamespace string, client client.Client) (*k8upv1.Snapshot, error)
	List(ctx context.Context, instanceNamespace string, client client.Client) (*k8upv1.SnapshotList, error)
	Watch(ctx context.Context, namespace string, options *metainternalversion.ListOptions, scClient client.WithWatch) (watch.Interface, error)
	GetFromEvent(in watch.Event) (*k8upv1.Snapshot, error)
}

// ConcreteSnapshotHandler implements Snapshothandler.
type ConcreteSnapshotHandler struct {
	client client.WithWatch
}

// Get returns the snapshot with the given ID.
func (c *ConcreteSnapshotHandler) Get(ctx context.Context, id, instanceNamespace string, kubeclient client.Client) (*k8upv1.Snapshot, error) {

	snapshot := &k8upv1.Snapshot{}
	err := kubeclient.Get(ctx, client.ObjectKey{Namespace: instanceNamespace, Name: id}, snapshot)

	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// List returns all snapshots in the given namespace.
func (c *ConcreteSnapshotHandler) List(ctx context.Context, instanceNamespace string, kubeclient client.Client) (*k8upv1.SnapshotList, error) {

	snapshots := &k8upv1.SnapshotList{}
	err := kubeclient.List(ctx, snapshots, &client.ListOptions{Namespace: instanceNamespace})

	if err != nil {
		return nil, err
	}

	return snapshots, nil
}

// Watch returns a watcher for the given objects
func (c *ConcreteSnapshotHandler) Watch(ctx context.Context, namespace string, options *metainternalversion.ListOptions, kubeclient client.WithWatch) (watch.Interface, error) {

	snapshots, err := c.List(ctx, namespace, kubeclient)
	if err != nil {
		return nil, err
	}
	return kubeclient.Watch(ctx, snapshots)
}

// GetFromEvent resolves watch.Event into k8upv1.Snapshot
func (c *ConcreteSnapshotHandler) GetFromEvent(in watch.Event) (*k8upv1.Snapshot, error) {
	snap, ok := in.Object.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("cannot parse snapshot from watch event")
	}

	snapshot := &k8upv1.Snapshot{}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(snap.Object, snapshot)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// New returns a new Snapshothandler implemented by ConcreteSnapshotHandler.
func New(client client.WithWatch) Snapshothandler {
	return &ConcreteSnapshotHandler{
		client: client,
	}
}
