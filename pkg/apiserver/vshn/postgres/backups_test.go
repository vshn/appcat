package postgres

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	kindSgBackup   = "sgbackups"
	kindCnpgBackup = "backups"
)

func TestConvertToBackupInfo(t *testing.T) {
	tests := map[string]struct {
		unstr      *unstructured.Unstructured
		backupInfo *v1.BackupInfo
		err        error
	}{
		// StackGres
		"GivenAnUnstructuredStackGresObject_ThenBackupInfo": {
			unstr:      getUnstructuredObjectSg("one"),
			backupInfo: getBackupInfoSg("one"),
			err:        nil,
		},
		"GivenAnUnstructuredStackGresObjectWithoutMeta_ThenError": {
			unstr:      getUnstructuredObjectWithoutMeta("two", kindSgBackup),
			backupInfo: nil,
			err:        fmt.Errorf("cannot parse metadata from object %s", getUnstructuredObjectWithoutMeta("two", kindSgBackup)),
		},
		"GivenAnUnstructuredStackGresObjectWithoutProcess_ThenBackupInfo": {
			unstr:      getUnstructuredObjectWithoutProcess("three"),
			backupInfo: getBackupInfoWithoutProcess("three", kindSgBackup),
			err:        nil,
		},
		"GivenAnUnstructuredStackGresObjectWithoutBI_ThenBackupInfo": {
			unstr:      getUnstructuredStackGresObjectWithoutBI("four"),
			backupInfo: getBackupInfoWithoutBI("four", kindSgBackup),
			err:        nil,
		},

		// CNPG
		"GivenAnUnstructuredCNPGObject_ThenBackupInfo": {
			unstr:      getUnstructuredObjectCnp("one"),
			backupInfo: getBackupInfoCnpg("one"),
			err:        nil,
		},
		"GivenAnUnstructuredCNPGObjectWithoutMeta_ThenError": {
			unstr:      getUnstructuredObjectWithoutMeta("two", kindCnpgBackup),
			backupInfo: nil,
			err:        fmt.Errorf("cannot parse metadata from object %s", getUnstructuredObjectWithoutMeta("two", kindCnpgBackup)),
		},
	}

	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			actual, err := convertToBackupInfo(tc.unstr)

			if tc.err != nil {
				assert.EqualError(t, err, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.backupInfo, actual)
		})
	}
}

func getBackupInfoWithoutProcess(name, kind string) *v1.BackupInfo {
	var b *v1.BackupInfo
	if kind == kindSgBackup {
		b = getBackupInfoSg(name)
	} else {
		b = getBackupInfoCnpg(name)
	}

	b.Process = runtime.RawExtension{}
	return b
}

func getBackupInfoWithoutBI(name, kind string) *v1.BackupInfo {
	var b *v1.BackupInfo
	if kind == kindSgBackup {
		b = getBackupInfoSg(name)
	} else {
		b = getBackupInfoCnpg(name)
	}

	b.BackupInformation = runtime.RawExtension{}
	return b
}

func getUnstructuredObjectWithoutMeta(name, kind string) *unstructured.Unstructured {
	var unstr *unstructured.Unstructured
	if kind == kindSgBackup {
		unstr = getUnstructuredObjectSg(name)
	} else {
		unstr = getUnstructuredObjectCnp(name)
	}

	unstructured.RemoveNestedField(unstr.Object, "metadata")
	return unstr
}

func getUnstructuredObjectWithoutProcess(name string) *unstructured.Unstructured {
	unstr := getUnstructuredObjectSg(name)
	unstructured.RemoveNestedField(unstr.Object, "status", "process")
	return unstr
}

func getUnstructuredStackGresObjectWithoutBI(name string) *unstructured.Unstructured {
	unstr := getUnstructuredObjectSg(name)
	unstructured.RemoveNestedField(unstr.Object, "status", "backupInformation")
	return unstr
}

func getBackupInfoSg(name string) *v1.BackupInfo {
	return &v1.BackupInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "namespaceOne",
		},
		Process: runtime.RawExtension{Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"status": "Success",
				"err":    "",
			},
		}},
		BackupInformation: runtime.RawExtension{Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"memoryUsed": "1Gb",
				"cpuUsed":    "1",
				"finished":   true,
				"paths": map[string]interface{}{
					"pathOne": "/path/to/one",
					"pathTwo": "/path/to/two",
				},
			},
		}},
	}
}

func getBackupInfoCnpg(name string) *v1.BackupInfo {
	return &v1.BackupInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "namespaceOne",
		},
		Process: runtime.RawExtension{Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"timing": map[string]interface{}{
					"start": "2024-06-01T12:00:00Z",
					"end":   "2024-06-01T12:05:00Z",
				},
				"jobPod": "backup-job-pod-123",
				"status": "Completed",
			},
		}},
		BackupInformation: runtime.RawExtension{Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"backupId":        "backup-123",
				"backupName":      "daily-backup",
				"destinationPath": "/backups/2024-06-01",
				"beginLSN":        "0/7000028",
				"beginWal":        "000000010000000700000028",
				"endLSN":          "0/7000030",
				"endWal":          "000000010000000700000030",
			},
		}},
	}
}

func getUnstructuredObjectSg(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": kindSgBackup,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "namespaceOne",
			},
			"status": map[string]interface{}{
				"process": map[string]interface{}{
					"status": "Success",
					"err":    "",
				},
				"nonRelevantField": true,
				"backupInformation": map[string]interface{}{
					"memoryUsed": "1Gb",
					"cpuUsed":    "1",
					"finished":   true,
					"paths": map[string]interface{}{
						"pathOne": "/path/to/one",
						"pathTwo": "/path/to/two",
					},
				},
			},
		},
	}
}

func getUnstructuredObjectCnp(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": kindCnpgBackup,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "namespaceOne",
			},
			"status": map[string]interface{}{
				"instanceID": map[string]interface{}{
					"podName": "backup-job-pod-123",
				},
				"startedAt":       "2024-06-01T12:00:00Z",
				"stoppedAt":       "2024-06-01T12:05:00Z",
				"phase":           "Completed",
				"backupId":        "backup-123",
				"backupName":      "daily-backup",
				"destinationPath": "/backups/2024-06-01",
				"beginLSN":        "0/7000028",
				"beginWal":        "000000010000000700000028",
				"endLSN":          "0/7000030",
				"endWal":          "000000010000000700000030",
			},
		},
	}
}
