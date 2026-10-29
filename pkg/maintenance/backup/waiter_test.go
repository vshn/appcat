package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"github.com/vshn/appcat/v4/pkg"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testNs = "test-ns"

func newTestWaiter(c client.Client, inProgress InProgressFunc, leftoverType client.Object) *Waiter {
	w := NewWaiter(c, inProgress, leftoverType, logr.Discard())
	w.timeout = 500 * time.Millisecond
	w.interval = 50 * time.Millisecond
	return w
}

func cnpgCluster(instances int) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: testNs},
		Spec:       cnpgv1.ClusterSpec{Instances: instances},
	}
}

func cnpgBackup(name string, phase cnpgv1.BackupPhase) *cnpgv1.Backup {
	return &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNs},
		Status:     cnpgv1.BackupStatus{Phase: phase},
	}
}

func TestWaiter_WaitForRunningBackups(t *testing.T) {
	tests := []struct {
		name        string
		objs        []client.Object
		finishAfter time.Duration
		wantErr     string
	}{
		{
			name: "GivenNoBackupRunning_ThenReturn",
			objs: []client.Object{cnpgCluster(1), cnpgBackup("daily", cnpgv1.BackupPhaseCompleted)},
		},
		{
			name:        "GivenBackupRunning_WhenItFinishes_ThenReturn",
			objs:        []client.Object{cnpgCluster(1), cnpgBackup("daily", cnpgv1.BackupPhaseRunning)},
			finishAfter: 150 * time.Millisecond,
		},
		{
			name:    "GivenBackupRunning_WhenItDoesNotFinish_ThenFail",
			objs:    []client.Object{cnpgCluster(1), cnpgBackup("daily", cnpgv1.BackupPhaseRunning)},
			wantErr: "backups still in progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("INSTANCE_NAMESPACE", testNs)

			c := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.objs...).
				WithStatusSubresource(&cnpgv1.Backup{}).
				Build()

			if tt.finishAfter > 0 {
				go func() {
					time.Sleep(tt.finishAfter)
					b := &cnpgv1.Backup{}
					if err := c.Get(context.Background(), client.ObjectKey{Name: "daily", Namespace: testNs}, b); err != nil {
						return
					}
					b.Status.Phase = cnpgv1.BackupPhaseCompleted
					_ = c.Status().Update(context.Background(), b)
				}()
			}

			err := newTestWaiter(c, CNPGBackupsInProgress, &cnpgv1.Backup{}).WaitForRunningBackups(context.Background())
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWaiter_WaitForRunningBackups_GivenNoNamespace_ThenFail(t *testing.T) {
	viper.Reset()
	c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).Build()

	err := newTestWaiter(c, CNPGBackupsInProgress, &cnpgv1.Backup{}).WaitForRunningBackups(context.Background())
	assert.ErrorContains(t, err, "INSTANCE_NAMESPACE not set")
}

func TestWaiter_WaitForRunningBackups_RemovesPreMaintenanceLeftovers(t *testing.T) {
	viper.Reset()
	viper.Set("INSTANCE_NAMESPACE", testNs)

	leftover := cnpgBackup("premaint-20240101-000000", cnpgv1.BackupPhasePending)
	leftover.Labels = map[string]string{BackupTypeLabelKey: BackupTypePreMaintenance}
	daily := cnpgBackup("daily", cnpgv1.BackupPhaseCompleted)
	otherNs := cnpgBackup("premaint-20240101-000000", cnpgv1.BackupPhasePending)
	otherNs.Namespace = "other-ns"
	otherNs.Labels = leftover.Labels

	c := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(cnpgCluster(1), leftover, daily, otherNs).
		Build()

	// The pending leftover would block the maintenance if it wasn't removed
	err := newTestWaiter(c, CNPGBackupsInProgress, &cnpgv1.Backup{}).WaitForRunningBackups(context.Background())
	require.NoError(t, err)

	assertGone(t, c, leftover)
	assertExists(t, c, daily)
	assertExists(t, c, otherNs)
}

func TestCNPGBackupsInProgress(t *testing.T) {
	tests := []struct {
		name string
		objs []client.Object
		want []string
	}{
		{
			name: "GivenNoCluster_ThenNothingRunning",
			objs: []client.Object{cnpgBackup("daily", cnpgv1.BackupPhaseRunning)},
		},
		{
			name: "GivenSuspendedCluster_ThenNothingRunning",
			objs: []client.Object{cnpgCluster(0), cnpgBackup("daily", cnpgv1.BackupPhasePending)},
		},
		{
			name: "GivenBackupsInAllPhases_ThenReturnTheUnfinishedOnes",
			objs: []client.Object{
				cnpgCluster(1),
				cnpgBackup("new", ""),
				cnpgBackup("pending", cnpgv1.BackupPhasePending),
				cnpgBackup("running", cnpgv1.BackupPhaseRunning),
				cnpgBackup("completed", cnpgv1.BackupPhaseCompleted),
				cnpgBackup("failed", cnpgv1.BackupPhaseFailed),
				cnpgBackup("walfailing", cnpgBackupPhaseWalArchivingFailing),
			},
			want: []string{"new", "pending", "running"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(tt.objs...).Build()

			got, err := CNPGBackupsInProgress(context.Background(), c, testNs, logr.Discard())
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestK8upBackupsInProgress(t *testing.T) {
	k8upBackup := func(name string, conditions ...metav1.Condition) *k8upv1.Backup {
		return &k8upv1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNs},
			Status:     k8upv1.Status{Conditions: conditions},
		}
	}
	completed := func(reason k8upv1.ConditionReason) metav1.Condition {
		return metav1.Condition{
			Type:   k8upv1.ConditionCompleted.String(),
			Status: metav1.ConditionTrue,
			Reason: reason.String(),
		}
	}

	c := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			k8upBackup("new"),
			k8upBackup("succeeded", completed(k8upv1.ReasonSucceeded)),
			k8upBackup("failed", completed(k8upv1.ReasonFailed)),
		).
		Build()

	got, err := K8upBackupsInProgress(context.Background(), c, testNs, logr.Discard())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"new"}, got)
}

func TestStackGresBackupsInProgress(t *testing.T) {
	cluster := &stackgresv1.SGCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: testNs},
		Spec:       stackgresv1.SGClusterSpec{Instances: 1},
	}
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup-cronjob",
			Namespace: testNs,
			UID:       "cronjob-uid",
			Labels:    map[string]string{"stackgres.io/scheduled-backup": "true"},
		},
	}
	job := func(name string, owned bool, conditions ...batchv1.JobCondition) *batchv1.Job {
		j := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNs},
			Status:     batchv1.JobStatus{Conditions: conditions},
		}
		if owned {
			j.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "CronJob",
				Name:       cronJob.Name,
				UID:        cronJob.UID,
			}}
		}
		return j
	}
	condition := func(t batchv1.JobConditionType) batchv1.JobCondition {
		return batchv1.JobCondition{Type: t, Status: corev1.ConditionTrue}
	}

	tests := []struct {
		name string
		objs []client.Object
		want []string
	}{
		{
			name: "GivenNoCluster_ThenNothingRunning",
			objs: []client.Object{cronJob, job("running", true)},
		},
		{
			name: "GivenSuspendedCluster_ThenNothingRunning",
			objs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: testNs},
					Spec:       stackgresv1.SGClusterSpec{Instances: 0},
				},
				cronJob, job("running", true),
			},
		},
		{
			name: "GivenJobs_ThenReturnTheUnfinishedBackupJobs",
			objs: []client.Object{
				cluster, cronJob,
				job("running", true),
				job("complete", true, condition(batchv1.JobComplete)),
				job("failed", true, condition(batchv1.JobFailed)),
				job("other-job", false),
			},
			want: []string{"running"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(tt.objs...).Build()

			got, err := StackGresBackupsInProgress(context.Background(), c, testNs, logr.Discard())
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
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
