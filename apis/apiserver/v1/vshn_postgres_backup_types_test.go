package v1

import (
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewVSHNBackupFromBackInfo(t *testing.T) {
	tests := map[string]struct {
		db                 string
		namespace          string
		backupInfo         *BackupInfo
		vshnPostgresBackup *VSHNPostgresBackup
	}{
		"GivenNoBackupInfo_ThenNil": {
			db:        "db1",
			namespace: "namespace",
		},
		"GivenNoDB_ThenNil": {
			backupInfo: &BackupInfo{
				ObjectMeta:        metav1.ObjectMeta{Name: "backup"},
				Process:           *rawFromObject(getTestProcess()),
				BackupInformation: *rawFromObject(getTestBI()),
			},
			namespace: "namespace",
		},
		"GivenNoNamespace_ThenNil": {
			backupInfo: &BackupInfo{
				ObjectMeta:        metav1.ObjectMeta{Name: "backup"},
				Process:           *rawFromObject(getTestProcess()),
				BackupInformation: *rawFromObject(getTestBI()),
			},
			db: "db1",
		},
		"GivenBackupInfo_ThenVSHNBackup": {
			db:        "db1",
			namespace: "namespace",
			backupInfo: &BackupInfo{
				ObjectMeta:        metav1.ObjectMeta{Name: "backup"},
				Process:           *rawFromObject(getTestProcess()),
				BackupInformation: *rawFromObject(getTestBI()),
			},
			vshnPostgresBackup: &VSHNPostgresBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "namespace"},
				Status: VSHNPostgresBackupStatus{
					DatabaseInstance:  "db1",
					Process:           rawFromObject(getTestProcess()),
					BackupInformation: rawFromObject(getTestBI()),
				},
			},
		},
		"GivenBackupInfoWithNoProcess_ThenVSHNBackup": {
			db:        "db1",
			namespace: "namespace",
			backupInfo: &BackupInfo{
				ObjectMeta:        metav1.ObjectMeta{Name: "backup"},
				BackupInformation: *rawFromObject(getTestBI()),
			},
			vshnPostgresBackup: &VSHNPostgresBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "namespace"},
				Status: VSHNPostgresBackupStatus{
					DatabaseInstance:  "db1",
					BackupInformation: rawFromObject(getTestBI()),
				},
			},
		},
		"GivenBackupInfoWithNoBI_ThenVSHNBackup": {
			db:        "db1",
			namespace: "namespace",
			backupInfo: &BackupInfo{
				ObjectMeta: metav1.ObjectMeta{Name: "backup"},
				Process:    *rawFromObject(getTestProcess()),
			},
			vshnPostgresBackup: &VSHNPostgresBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "namespace"},
				Status: VSHNPostgresBackupStatus{
					DatabaseInstance: "db1",
					Process:          rawFromObject(getTestProcess()),
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actualBackup := NewVSHNPostgresBackup(tt.backupInfo, tt.db, tt.namespace)
			assert.DeepEqual(t, tt.vshnPostgresBackup, actualBackup)
		})
	}
}

func getTestProcess() unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": "Completed",
		},
	}
}

func getTestBI() unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"memory": 1024,
			"time":   "45s",
		},
	}
}

func rawFromObject(object unstructured.Unstructured) *runtime.RawExtension {
	return &runtime.RawExtension{Object: &object}
}
