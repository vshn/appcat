package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/appcat/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_vshnRedisBackupStorage_ConvertToTable_noduplicate(t *testing.T) {
	v := &vshnRedisBackupStorage{}
	obj := vshnv1.VSHNRedisBackupList{
		Items: []vshnv1.VSHNRedisBackup{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo2",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo3",
				},
			},
		},
	}
	got, err := v.ConvertToTable(context.TODO(), &obj, nil)
	assert.NoError(t, err)
	assert.Len(t, got.Rows, 3)

	foo1, ok := got.Rows[0].Object.Object.(*vshnv1.VSHNRedisBackup)
	if assert.True(t, ok, "unexpected type for foo1") {
		assert.Equal(t, "foo1", foo1.Namespace)
	}
	foo2, ok := got.Rows[1].Object.Object.(*vshnv1.VSHNRedisBackup)
	if assert.True(t, ok, "unexpected type for foo1") {
		assert.Equal(t, "foo2", foo2.Namespace)
	}
	foo3, ok := got.Rows[2].Object.Object.(*vshnv1.VSHNRedisBackup)
	if assert.True(t, ok, "unexpected type for foo1") {
		assert.Equal(t, "foo3", foo3.Namespace)
	}

}
