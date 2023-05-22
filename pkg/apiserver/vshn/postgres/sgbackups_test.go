package postgres

import (
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/apis/appcat/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestConvertToBackupInfo(t *testing.T) {
	tests := map[string]struct {
		unstr      *unstructured.Unstructured
		backupInfo *v1.SGBackupInfo
		err        error
	}{
		"GivenAnUnstructuredObject_ThenBackupInfo": {
			unstr:      getUnstructuredObject("one"),
			backupInfo: getBackupInfo("one"),
			err:        nil,
		},
		"GivenAnUnstructuredObjectWithoutMeta_ThenError": {
			unstr:      getUnstructuredObjectWithoutMeta("two"),
			backupInfo: nil,
			err:        fmt.Errorf("cannot parse metadata from object %s", getUnstructuredObjectWithoutMeta("two")),
		},
		"GivenAnUnstructuredObjectWithoutProcess_ThenBackupInfo": {
			unstr:      getUnstructuredObjectWithoutProcess("three"),
			backupInfo: getBackupInfoWithoutProcess("three"),
			err:        nil,
		},
		"GivenAnUnstructuredObjectWithoutBI_ThenBackupInfo": {
			unstr:      getUnstructuredObjectWithoutBI("four"),
			backupInfo: getBackupInfoWithoutBI("four"),
			err:        nil,
		},
	}

	for n, tc := range tests {

		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			actual, err := convertToSGBackupInfo(tc.unstr)

			if tc.err != nil {
				assert.EqualError(t, err, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.backupInfo, actual)
		})
	}
}

func getBackupInfoWithoutProcess(name string) *v1.SGBackupInfo {
	b := getBackupInfo(name)
	b.Process = runtime.RawExtension{}
	return b
}

func getBackupInfoWithoutBI(name string) *v1.SGBackupInfo {
	b := getBackupInfo(name)
	b.BackupInformation = runtime.RawExtension{}
	return b
}

func getUnstructuredObjectWithoutMeta(name string) *unstructured.Unstructured {
	unstr := getUnstructuredObject(name)
	unstructured.RemoveNestedField(unstr.Object, "metadata")
	return unstr
}

func getUnstructuredObjectWithoutProcess(name string) *unstructured.Unstructured {
	unstr := getUnstructuredObject(name)
	unstructured.RemoveNestedField(unstr.Object, "status", "process")
	return unstr
}

func getUnstructuredObjectWithoutBI(name string) *unstructured.Unstructured {
	unstr := getUnstructuredObject(name)
	unstructured.RemoveNestedField(unstr.Object, "status", "backupInformation")
	return unstr
}

func getBackupInfo(name string) *v1.SGBackupInfo {
	return &v1.SGBackupInfo{
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

func getUnstructuredObject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
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
