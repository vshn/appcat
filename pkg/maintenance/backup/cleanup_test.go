package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"github.com/vshn/appcat/v4/pkg"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCleanupPreviousBackups(t *testing.T) {
	stale := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "premaint-20240101-000000",
			Namespace: "test-ns",
			Labels:    PreMaintenanceLabels(),
		},
	}
	scheduled := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduled-backup",
			Namespace: "test-ns",
		},
	}
	otherNs := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "premaint-20240101-000000",
			Namespace: "other-ns",
			Labels:    PreMaintenanceLabels(),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(stale, scheduled, otherNs).
		Build()

	err := CleanupPreviousBackups(context.Background(), fakeClient, &cnpgv1.Backup{}, "test-ns", logr.Discard())
	require.NoError(t, err)

	assertGone(t, fakeClient, stale)
	assertExists(t, fakeClient, scheduled)
	assertExists(t, fakeClient, otherNs)
}

func TestK8upBackupRunner_RunBackup_RemovesPreviousBackups(t *testing.T) {
	stale := &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "premaint-20240101-000000",
			Namespace: "test-ns",
			Labels:    PreMaintenanceLabels(),
		},
	}
	schedule := &k8upv1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "test-ns",
		},
		Spec: k8upv1.ScheduleSpec{
			Backend: &k8upv1.Backend{},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(stale, schedule).
		WithStatusSubresource(&k8upv1.Backup{}).
		Build()

	runner := &K8upBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   500 * time.Millisecond,
		},
	}

	// The watch times out because nothing sets the status, that is expected here
	_ = runner.RunBackup(context.Background(), "test-ns", "premaint-20250101-000000")

	assertGone(t, fakeClient, stale)
	assertExists(t, fakeClient, &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "premaint-20250101-000000", Namespace: "test-ns"},
	})
}

func TestCNPGBackupRunner_RunBackup_RemovesPreviousBackups(t *testing.T) {
	stale := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "premaint-20240101-000000",
			Namespace: "test-ns",
			Labels:    PreMaintenanceLabels(),
		},
	}
	cluster := &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
		},
		Spec: cnpgv1.ClusterSpec{Instances: 1},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(stale, cluster).
		WithStatusSubresource(&cnpgv1.Backup{}).
		Build()

	runner := &CNPGBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   500 * time.Millisecond,
		},
	}

	_ = runner.RunBackup(context.Background(), "test-ns", "premaint-20250101-000000")

	assertGone(t, fakeClient, stale)
	assertExists(t, fakeClient, &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "premaint-20250101-000000", Namespace: "test-ns"},
	})
}

func TestStackGresBackupRunner_RunBackup_RemovesPreviousJobs(t *testing.T) {
	stale := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "premaint-20240101-000000",
			Namespace: "test-ns",
			Labels:    PreMaintenanceLabels(),
		},
	}
	scheduled := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduled-backup-job",
			Namespace: "test-ns",
		},
	}
	cluster := &stackgresv1.SGCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
		},
		Spec: stackgresv1.SGClusterSpec{Instances: 1},
	}
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup-cronjob",
			Namespace: "test-ns",
			Labels: map[string]string{
				"stackgres.io/scheduled-backup": "true",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(stale, scheduled, cluster, cronJob).
		WithStatusSubresource(&batchv1.Job{}).
		Build()

	runner := &StackGresBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   500 * time.Millisecond,
		},
	}

	_ = runner.RunBackup(context.Background(), "test-ns", "premaint-20250101-000000")

	assertGone(t, fakeClient, stale)
	assertExists(t, fakeClient, scheduled)
	assertExists(t, fakeClient, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "premaint-20250101-000000", Namespace: "test-ns"},
	})
}

func assertGone(t *testing.T, c client.Client, obj client.Object) {
	t.Helper()
	err := c.Get(context.Background(), client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	assert.True(t, apierrors.IsNotFound(err), "expected %s to be gone, got %v", obj.GetName(), err)
}

func assertExists(t *testing.T, c client.Client, obj client.Object) {
	t.Helper()
	err := c.Get(context.Background(), client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	assert.NoError(t, err, "expected %s to exist", obj.GetName())
}
