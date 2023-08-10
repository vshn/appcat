package postgres

import (
	"github.com/vshn/appcat/v4/apis/appcat/v1"
	"github.com/vshn/appcat/v4/test/mocks"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"testing"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newMockedVSHNPostgresBackupStorage is a mocked instance of vshnPostgresBackup
func newMockedVSHNPostgresBackupStorage(t *testing.T, ctrl *gomock.Controller) (rest.StandardStorage, *mocks.MocksgbackupProvider, *mocks.MockvshnPostgresqlProvider) {
	t.Helper()
	sgbackup := mocks.NewMocksgbackupProvider(ctrl)
	vshnpostgres := mocks.NewMockvshnPostgresqlProvider(ctrl)
	stor := &vshnPostgresBackupStorage{
		sgbackups:      sgbackup,
		vshnpostgresql: vshnpostgres,
	}
	return rest.Storage(stor).(rest.StandardStorage), sgbackup, vshnpostgres
}

// Test AppCat instances
var (
	vshnBackupOne = &v1.VSHNPostgresBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one",
			Namespace: "namespace",
		},
		Status: v1.VSHNPostgresBackupStatus{
			DatabaseInstance:  "postgres-one",
			Process:           &runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"status": "Failed"}}},
			BackupInformation: &runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"disk": "1GB", "cpu": "1"}}},
		},
	}

	backupInfoOne = &v1.SGBackupInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "one",
			Namespace: "namespace-one",
		},
		Process:           runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"status": "Failed"}}},
		BackupInformation: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"disk": "1GB", "cpu": "1"}}},
	}

	unstructuredBackupOne = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "one",
				"namespace": "namespace-one",
			},
			"status": map[string]interface{}{
				"process": map[string]interface{}{
					"status": "Failed",
				},
				"backupInformation": map[string]interface{}{
					"disk": "1GB",
					"cpu":  "1",
				},
			},
		},
	}

	vshnBackupTwo = &v1.VSHNPostgresBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two",
			Namespace: "namespace",
		},
		Status: v1.VSHNPostgresBackupStatus{
			DatabaseInstance:  "postgres-two",
			Process:           &runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"status": "Completed"}}},
			BackupInformation: &runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"disk": "2GB", "cpu": "2"}}},
		},
	}

	backupInfoTwo = &v1.SGBackupInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "two",
			Namespace: "namespace-two",
		},
		Process:           runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"status": "Completed"}}},
		BackupInformation: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{"disk": "2GB", "cpu": "2"}}},
	}

	unstructuredBackupTwo = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "two",
				"namespace": "namespace-two",
			},
			"status": map[string]interface{}{
				"process": map[string]interface{}{
					"status": "Completed",
				},
				"backupInformation": map[string]interface{}{
					"disk": "2GB",
					"cpu":  "2",
				},
			},
		},
	}

	vshnPostgreSQLInstances = &vshnv1.XVSHNPostgreSQLList{
		Items: []vshnv1.XVSHNPostgreSQL{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "postgres-one-tty",
					Labels: map[string]string{
						claimNameLabel:      "postgres-one",
						claimNamespaceLabel: "namespace-claim",
					},
				},
				Status: vshnv1.VSHNPostgreSQLStatus{
					InstanceNamespace: "namespace-one",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "postgres-two-bbf",
					Labels: map[string]string{
						claimNameLabel:      "postgres-two",
						claimNamespaceLabel: "namespace-claim",
					},
				},
				Status: vshnv1.VSHNPostgreSQLStatus{
					InstanceNamespace: "namespace-two",
				},
			},
		},
	}
)
