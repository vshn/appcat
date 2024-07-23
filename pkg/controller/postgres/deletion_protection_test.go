package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/jsonpatch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	_ = vshnv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
}

func Test_Handle(t *testing.T) {
	tests := map[string]struct {
		ctx           context.Context
		obj           vshnv1.XVSHNPostgreSQL
		expectedPatch client.Patch
	}{

		"WhenFinalizer_ThenRemoveOpPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(1),
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
		"WhenMultipleFinalzers_ThenOnlyKeepOne": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(2),
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
		"WhenMoreFinalzers_ThenOnlyKeepOne": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(3),
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// WHEN
			actualPatch, err := handle(tc.ctx, &tc.obj)

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPatch, actualPatch)
		})
	}
}

func Test_GetPatchObjectFinalizer(t *testing.T) {
	tests := map[string]struct {
		obj           vshnv1.XVSHNPostgreSQL
		op            jsonpatch.JSONop
		expectedPatch client.Patch
	}{
		"WhenOpRemove_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(1),
			op:            jsonpatch.JSONopRemove,
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			log := logr.Discard()

			// WHEN
			patch, err := getPatchObjectFinalizer(log, &tc.obj, tc.op)

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPatch, patch)

		})
	}
}

func getXVSHNPostgreSQL(addFinalizer int) vshnv1.XVSHNPostgreSQL {
	obj := vshnv1.XVSHNPostgreSQL{}
	for i := 0; i < addFinalizer; i++ {
		obj.Finalizers = append(obj.Finalizers, finalizerName)
	}
	return obj
}

func getPatch(op jsonpatch.JSONop) client.Patch {
	strIndex := strconv.Itoa(0)
	if op == jsonpatch.JSONopAdd {
		strIndex = "-"
	}
	patchOps := []jsonpatch.JSONpatch{
		{
			Op:    op,
			Path:  "/metadata/finalizers/" + strIndex,
			Value: finalizerName,
		},
	}
	patch, _ := json.Marshal(patchOps)
	return client.RawPatch(types.JSONPatchType, patch)
}
