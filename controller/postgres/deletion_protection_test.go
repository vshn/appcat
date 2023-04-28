package postgres

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/component-appcat/apis/vshn/v1"
	apis "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"testing"
	"time"
)

func Test_Handle(t *testing.T) {
	previousDay := getCurrentTime().AddDate(0, 0, -1)
	tests := map[string]struct {
		ctx           context.Context
		obj           v1.XVSHNPostgreSQL
		enabled       bool
		retention     int
		expectedPatch client.Patch
	}{
		"WhenRetentionDisableAndNoFinalizer_ThenNilPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(false, nil),
			enabled:       false,
			retention:     0,
			expectedPatch: nil,
		},
		"WhenRetentionDisableAndFinalizer_ThenRemoveOpPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(true, nil),
			enabled:       false,
			retention:     0,
			expectedPatch: getPatch(opRemove),
		},
		"WhenRetentionEnabledAndNoFinalizerAndNotDeleted_ThenAddOpPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(false, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: getPatch(opAdd),
		},
		"WhenRetentionEnabledAndFinalizerAndDeletedAndRetentionNonZero_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(true, &previousDay),
			enabled:       true,
			retention:     1,
			expectedPatch: nil,
		},
		"WhenRetentionEnabledAndFinalizerAndDeletedAndRetentionZero_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(true, &previousDay),
			enabled:       true,
			retention:     0,
			expectedPatch: getPatch(opRemove),
		},
		"WhenRetentionEnabledAndFinalizerAndNotDeleted_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(true, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// WHEN
			actualPatch, err := handle(tc.ctx, &tc.obj, tc.enabled, tc.retention)

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPatch, actualPatch)
		})
	}
}

func Test_CheckRetention(t *testing.T) {
	twoDaysBefore := getCurrentTime().AddDate(0, 0, -2)
	previousDay := getCurrentTime().AddDate(0, 0, -1)
	nextDay := getCurrentTime().AddDate(0, 0, 1)
	tests := map[string]struct {
		ctx        context.Context
		obj        v1.XVSHNPostgreSQL
		retention  int
		expectedOp jsonOp
	}{
		"WhenDeletionTimePlusRetentionTimeBeforeCurrentRuntime_ThenRemoveOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(true, &twoDaysBefore),
			retention:  1,
			expectedOp: opRemove,
		},
		"WhenDeletionTimeAndNoRetentionBeforeCurrentRuntime_ThenRemoveOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(true, &twoDaysBefore),
			retention:  0,
			expectedOp: opRemove,
		},
		"WhenDeletionTimePlusRetentionAfterCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(true, &twoDaysBefore),
			retention:  5,
			expectedOp: opNone,
		},
		"WhenDeletionTimeWithoutRetentionAfterCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(true, &nextDay),
			retention:  5,
			expectedOp: opNone,
		},
		"WhenDeletionTimeWithRetentionEqualsCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(true, &previousDay),
			retention:  1,
			expectedOp: opNone,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// WHEN
			actualOp := checkRetention(tc.ctx, &tc.obj, tc.retention)

			// THEN
			assert.Equal(t, tc.expectedOp, actualOp)
		})
	}
}

func Test_GetRequeueTime(t *testing.T) {
	previousDay := getCurrentTime().AddDate(0, 0, -1)
	tests := map[string]struct {
		ctx              context.Context
		obj              v1.XVSHNPostgreSQL
		deletionTime     *time.Time
		retention        int
		expectedDuration time.Duration
	}{
		"WhenNoDeletion_ThenReturnIn30Seconds": {
			ctx:              context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:              getXVSHNPostgreSQL(true, nil),
			deletionTime:     nil,
			retention:        1,
			expectedDuration: time.Second * 30,
		},
		"WhenDeletionTime_ThenCalculateDuration": {
			ctx:              context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:              getXVSHNPostgreSQL(true, &previousDay),
			deletionTime:     &previousDay,
			retention:        2,
			expectedDuration: time.Hour * 24,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			delTime := transformToK8sTime(tc.deletionTime)

			// WHEN
			actualDuration := getRequeueTime(tc.ctx, &tc.obj, delTime, tc.retention)

			// THEN
			assert.Equal(t, tc.expectedDuration, actualDuration)
		})
	}
}

func Test_GetPatchObjectFinalizer(t *testing.T) {
	tests := map[string]struct {
		obj           v1.XVSHNPostgreSQL
		op            jsonOp
		expectedPatch client.Patch
	}{
		"WhenOpNone_ThenNil": {
			obj:           getXVSHNPostgreSQL(true, nil),
			op:            opNone,
			expectedPatch: nil,
		},
		"WhenOpRemove_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(true, nil),
			op:            opRemove,
			expectedPatch: getPatch(opRemove),
		},
		"WhenOpAdd_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(true, nil),
			op:            opAdd,
			expectedPatch: getPatch(opAdd),
		},
		"WhenOpReplace_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(true, nil),
			op:            opReplace,
			expectedPatch: getPatch(opReplace),
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

func transformToK8sTime(t *time.Time) *apis.Time {
	if t != nil {
		temp := apis.NewTime(*t)
		return &temp
	}
	return nil
}

func getXVSHNPostgreSQL(addFinalizer bool, deletedTime *time.Time) v1.XVSHNPostgreSQL {
	obj := v1.XVSHNPostgreSQL{}
	if addFinalizer {
		obj.Finalizers = []string{finalizerName}
	}
	if deletedTime != nil {
		obj.SetDeletionTimestamp(&apis.Time{Time: *deletedTime})
	}
	return obj
}

func getPatch(op jsonOp) client.Patch {
	patchOps := []jsonpatch{
		{
			Op:    op,
			Path:  "/metadata/finalizers/" + strconv.Itoa(0),
			Value: finalizerName,
		},
	}
	patch, _ := json.Marshal(patchOps)
	return client.RawPatch(types.JSONPatchType, patch)
}
