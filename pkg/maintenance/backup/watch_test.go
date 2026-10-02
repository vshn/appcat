package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// newWatchableClientBuilder returns a fake client builder with the metadata.name field index
// that WatchUntilDone lists with. The API server supports it natively, the fake client
// needs an explicit index for each watched type.
func newWatchableClientBuilder(objs ...client.Object) *fake.ClientBuilder {
	b := fake.NewClientBuilder().WithScheme(pkg.SetupScheme())
	for _, obj := range objs {
		b = b.WithIndex(obj, "metadata.name", func(o client.Object) []string {
			return []string{o.GetName()}
		})
	}
	return b
}

func TestWatchUntilDone_GivenWatchClosedByServer_ThenReconnect(t *testing.T) {
	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: "test-ns",
		},
		Status: cnpgv1.BackupStatus{
			Phase: cnpgv1.BackupPhaseRunning,
		},
	}

	watchCalls := 0
	c := newWatchableClientBuilder(&cnpgv1.Backup{}).
		WithObjects(backup).
		WithStatusSubresource(backup).
		WithInterceptorFuncs(interceptor.Funcs{
			Watch: func(ctx context.Context, c client.WithWatch, obj client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
				watchCalls++
				if watchCalls == 1 {
					// Simulate the API server ending the watch while the backup is still running
					w := watch.NewFake()
					w.Stop()
					return w, nil
				}

				w, err := c.Watch(ctx, obj, opts...)
				if err != nil {
					return nil, err
				}
				// The backup finishes while the new watch is running
				go func() {
					done := backup.DeepCopy()
					assert.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(done), done))
					done.Status.Phase = cnpgv1.BackupPhaseCompleted
					assert.NoError(t, c.Status().Update(ctx, done))
				}()
				return w, nil
			},
		}).
		Build()

	runner := NewCNPGBackupRunner(c, logr.Discard())
	toWatch := &cnpgv1.Backup{}
	toWatch.SetName(backup.Name)
	toWatch.SetNamespace(backup.Namespace)

	err := WatchUntilDone(context.Background(), c, toWatch, &cnpgv1.BackupList{}, 10*time.Second, runner.checkDone, runner.checkSuccess, logr.Discard())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, watchCalls, 2)
}

func TestWatchUntilDone_GivenAlreadyDone_ThenReturnWithoutWatching(t *testing.T) {
	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: "test-ns",
		},
		Status: cnpgv1.BackupStatus{
			Phase: cnpgv1.BackupPhaseFailed,
			Error: "boom",
		},
	}

	c := newWatchableClientBuilder(&cnpgv1.Backup{}).
		WithObjects(backup).
		WithStatusSubresource(backup).
		Build()

	runner := NewCNPGBackupRunner(c, logr.Discard())
	toWatch := &cnpgv1.Backup{}
	toWatch.SetName(backup.Name)
	toWatch.SetNamespace(backup.Namespace)

	err := WatchUntilDone(context.Background(), c, toWatch, &cnpgv1.BackupList{}, 5*time.Second, runner.checkDone, runner.checkSuccess, logr.Discard())
	assert.ErrorContains(t, err, "backup failed: boom")
}
