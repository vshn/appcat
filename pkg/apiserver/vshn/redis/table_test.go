package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/appcat/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_vshnRedisBackupStorage_ConvertToTable(t *testing.T) {
	tests := []struct {
		name         string
		obj          runtime.Object
		tableOptions runtime.Object
		wantRows     int
		wantErr      bool
	}{
		{
			name:     "GivenOneBackup_ThenExpectOneRow",
			obj:      &vshnv1.VSHNRedisBackup{},
			wantRows: 1,
		},
		{
			name: "GivenMiltipleBackups_ThenExpectMultipleRows",
			obj: &vshnv1.VSHNRedisBackupList{
				Items: []vshnv1.VSHNRedisBackup{
					{},
					{},
					{},
				},
			},
			wantRows: 3,
		},
		{
			name:    "GivenNoBackupObject_ThenExpectError",
			obj:     &vshnv1.AppCat{},
			wantErr: true,
		},
		{
			name:    "GivenNil_ThenExpectError",
			obj:     nil,
			wantErr: true,
		},
		{
			name:    "GivenNonBackupList_THenExpectError",
			obj:     &vshnv1.VSHNPostgresBackupList{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &vshnRedisBackupStorage{}
			got, err := v.ConvertToTable(context.TODO(), tt.obj, tt.tableOptions)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			assert.Len(t, got.Rows, tt.wantRows)
		})
	}
}
