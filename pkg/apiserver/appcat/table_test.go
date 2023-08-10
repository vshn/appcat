package appcat

import (
	"context"
	"testing"

	v1 "github.com/vshn/appcat/v4/apis/appcat/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAppCatStorage_ConvertToTable(t *testing.T) {
	tests := map[string]struct {
		obj          runtime.Object
		tableOptions runtime.Object
		fail         bool
		nrRows       int
	}{
		"GivenEmptyAppCat_ThenSingleRow": {
			obj:    &v1.AppCat{},
			nrRows: 1,
		},
		"GivenAppCat_ThenSingleRow": {
			obj: &v1.AppCat{
				ObjectMeta: metav1.ObjectMeta{Name: "pippo"},

				Details: map[string]string{
					"zone":        "rma1",
					"displayname": "ObjectStorage",
				},
			},
			nrRows: 1,
		},
		"GivenAppCatList_ThenMultipleRow": {
			obj: &v1.AppCatList{
				Items: []v1.AppCat{
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
		"GivenNonAppCat_ThenFail": {
			obj:  &corev1.Pod{},
			fail: true,
		},
		"GivenNonAppCatList_ThenFail": {
			obj:  &corev1.PodList{},
			fail: true,
		},
	}
	appcatStore := &appcatStorage{}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			table, err := appcatStore.ConvertToTable(context.TODO(), tc.obj, tc.tableOptions)
			if tc.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, table.Rows, tc.nrRows)
		})
	}
}
