package redis

import (
	"context"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/k8up"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ vshnRedisProvider = &mockprovider{}
var _ k8up.Snapshothandler = &mockhandler{}

type mockprovider struct {
	err       error
	instances *vshnv1.VSHNRedisList
}

func (m *mockprovider) ListVSHNRedis(ctx context.Context, namespace string) (*vshnv1.VSHNRedisList, error) {

	instances := &vshnv1.VSHNRedisList{
		Items: []vshnv1.VSHNRedis{},
	}

	for _, instance := range m.instances.Items {
		if instance.GetNamespace() == namespace {
			instances.Items = append(instances.Items, instance)
		}
	}

	return instances, m.err
}

func (m *mockprovider) GetKubeConfig(ctx context.Context, instance vshnv1.VSHNRedis) ([]byte, error) {
	return nil, nil
}

func (m *mockprovider) GetKubeClient(ctx context.Context, instance vshnv1.VSHNRedis) (client.WithWatch, error) {
	return nil, nil
}

type mockhandler struct {
	snapshot  *k8upv1.Snapshot
	snapshots *k8upv1.SnapshotList
}

func (m *mockhandler) Get(ctx context.Context, id, instanceNamespace string, client client.Client) (*k8upv1.Snapshot, error) {
	return m.snapshot, nil
}

func (m *mockhandler) List(ctx context.Context, instanceNamespace string, client client.Client) (*k8upv1.SnapshotList, error) {

	snapshots := &k8upv1.SnapshotList{
		Items: []k8upv1.Snapshot{},
	}

	for _, snap := range m.snapshots.Items {
		if snap.GetNamespace() == instanceNamespace {
			snapshots.Items = append(snapshots.Items, snap)
		}
	}

	return snapshots, nil
}
func (m *mockhandler) Watch(ctx context.Context, namespace string, options *metainternalversion.ListOptions, client client.WithWatch) (watch.Interface, error) {
	return nil, nil
}
func (m *mockhandler) GetFromEvent(in watch.Event) (*k8upv1.Snapshot, error) {
	return nil, nil
}
