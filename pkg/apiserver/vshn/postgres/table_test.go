package postgres

import (
	"context"
	"github.com/vshn/appcat/apis/appcat/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestVSHNPostgresBackupStorage_ConvertToTable(t *testing.T) {
	tests := map[string]struct {
		obj          runtime.Object
		tableOptions runtime.Object
		fail         bool
		nrRows       int
	}{
		"GivenEmptyBackup_ThenSingleRow": {
			obj:    &v1.VSHNPostgresBackup{},
			nrRows: 1,
		},
		"GivenBackup_ThenSingleRow": {
			obj:    vshnBackupOne,
			nrRows: 1,
		},
		"GivenBackupList_ThenMultipleRow": {
			obj: &v1.VSHNPostgresBackupList{
				Items: []v1.VSHNPostgresBackup{
					{},
					{},
					{},
				},
			},
			nrRows: 3,
		},
		"GivenNil_ThenFail": {
			obj:  nil,
			fail: true,
		},
		"GivenNonBackup_ThenFail": {
			obj:  &corev1.Pod{},
			fail: true,
		},
		"GivenNonBackupList_ThenFail": {
			obj:  &corev1.PodList{},
			fail: true,
		},
	}
	backupStore := &vshnPostgresBackupStorage{}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			table, err := backupStore.ConvertToTable(context.TODO(), tc.obj, tc.tableOptions)
			if tc.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, table.Rows, tc.nrRows)
		})
	}
}
