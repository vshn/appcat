package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/jsonpatch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	currentTimeKey = "now"
)

func init() {
	_ = vshnv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
}

func Test_Handle(t *testing.T) {
	previousDay := getCurrentTime().AddDate(0, 0, -1)
	tests := map[string]struct {
		ctx           context.Context
		obj           vshnv1.XVSHNPostgreSQL
		enabled       bool
		retention     int
		expectedPatch client.Patch
	}{
		"WhenRetentionDisableAndNoFinalizer_ThenNilPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(0, nil),
			enabled:       false,
			retention:     0,
			expectedPatch: nil,
		},
		"WhenRetentionDisableAndFinalizer_ThenRemoveOpPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(1, nil),
			enabled:       false,
			retention:     0,
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
		"WhenRetentionEnabledAndNoFinalizerAndNotDeleted_ThenAddOpPatch": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(0, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: getPatch(jsonpatch.JSONopAdd),
		},
		"WhenRetentionEnabledAndFinalizerAndDeletedAndRetentionNonZero_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(1, &previousDay),
			enabled:       true,
			retention:     1,
			expectedPatch: nil,
		},
		"WhenRetentionEnabledAndFinalizerAndDeletedAndRetentionZero_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(1, &previousDay),
			enabled:       true,
			retention:     0,
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
		"WhenRetentionEnabledAndFinalizerAndNotDeleted_ThenNilPatch": {
			ctx:           context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:           getXVSHNPostgreSQL(1, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: nil,
		},
		"WhenMultipleFinalzers_ThenOnlyKeepOne": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(2, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: getRemovePatches(2),
		},
		"WhenMoreFinalzers_ThenOnlyKeepOne": {
			ctx:           context.Background(),
			obj:           getXVSHNPostgreSQL(3, nil),
			enabled:       true,
			retention:     0,
			expectedPatch: getRemovePatches(3),
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
		obj        vshnv1.XVSHNPostgreSQL
		retention  int
		expectedOp jsonpatch.JSONop
	}{
		"WhenDeletionTimePlusRetentionTimeBeforeCurrentRuntime_ThenRemoveOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(1, &twoDaysBefore),
			retention:  1,
			expectedOp: jsonpatch.JSONopRemove,
		},
		"WhenDeletionTimeAndNoRetentionBeforeCurrentRuntime_ThenRemoveOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(1, &twoDaysBefore),
			retention:  0,
			expectedOp: jsonpatch.JSONopRemove,
		},
		"WhenDeletionTimePlusRetentionAfterCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(1, &twoDaysBefore),
			retention:  5,
			expectedOp: jsonpatch.JSONopNone,
		},
		"WhenDeletionTimeWithoutRetentionAfterCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(1, &nextDay),
			retention:  5,
			expectedOp: jsonpatch.JSONopNone,
		},
		"WhenDeletionTimeWithRetentionEqualsCurrentRuntime_ThenNonOp": {
			ctx:        context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:        getXVSHNPostgreSQL(1, &previousDay),
			retention:  1,
			expectedOp: jsonpatch.JSONopNone,
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
		obj              vshnv1.XVSHNPostgreSQL
		deletionTime     *time.Time
		retention        int
		expectedDuration time.Duration
	}{
		"WhenNoDeletion_ThenReturnIn30Seconds": {
			ctx:              context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:              getXVSHNPostgreSQL(1, nil),
			deletionTime:     nil,
			retention:        1,
			expectedDuration: time.Second * 30,
		},
		"WhenDeletionTime_ThenCalculateDuration": {
			ctx:              context.WithValue(context.Background(), currentTimeKey, getCurrentTime()),
			obj:              getXVSHNPostgreSQL(1, &previousDay),
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
		obj           vshnv1.XVSHNPostgreSQL
		op            jsonpatch.JSONop
		expectedPatch client.Patch
	}{
		"WhenOpNone_ThenNil": {
			obj:           getXVSHNPostgreSQL(1, nil),
			op:            jsonpatch.JSONopNone,
			expectedPatch: nil,
		},
		"WhenOpRemove_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(1, nil),
			op:            jsonpatch.JSONopRemove,
			expectedPatch: getPatch(jsonpatch.JSONopRemove),
		},
		"WhenOpAdd_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(1, nil),
			op:            jsonpatch.JSONopAdd,
			expectedPatch: getPatch(jsonpatch.JSONopAdd),
		},
		"WhenOpReplace_ThenReturnPatch": {
			obj:           getXVSHNPostgreSQL(1, nil),
			op:            jsonpatch.JSONopReplace,
			expectedPatch: getPatch(jsonpatch.JSONopReplace),
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

func transformToK8sTime(t *time.Time) *metav1.Time {
	if t != nil {
		temp := metav1.NewTime(*t)
		return &temp
	}
	return nil
}

func getXVSHNPostgreSQL(addFinalizer int, deletedTime *time.Time) vshnv1.XVSHNPostgreSQL {
	obj := vshnv1.XVSHNPostgreSQL{}
	for i := 0; i < addFinalizer; i++ {
		obj.Finalizers = append(obj.Finalizers, finalizerName)
	}
	if deletedTime != nil {
		obj.SetDeletionTimestamp(&metav1.Time{Time: *deletedTime})
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

func getRemovePatches(count int) client.Patch {
	patchOps := []jsonpatch.JSONpatch{}
	for i := count; i > 0; i-- {
		if i == count {
			continue
		}
		patchOps = append(patchOps, jsonpatch.JSONpatch{
			Op:   jsonpatch.JSONopRemove,
			Path: "/metadata/finalizers/" + strconv.Itoa(i),
		})
	}
	patch, _ := json.Marshal(patchOps)
	return client.RawPatch(types.JSONPatchType, patch)
}

func Test_instanceNamespaceDeleted(t *testing.T) {
	tests := []struct {
		name          string
		want          jsonpatch.JSONop
		wantDeleted   bool
		wantFinalizer bool
		wantErr       bool
		instance      vshnv1.XVSHNPostgreSQL
		namespace     corev1.Namespace
		enabled       bool
	}{
		{
			name:          "GivenEnabledAndNotDeleted_ThenExpectNoOpAndFinalizer",
			want:          jsonpatch.JSONopNone,
			wantFinalizer: true,
			enabled:       true,
			instance: vshnv1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance-1",
				},
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
		},
		{
			name:          "GivenEnabledAndDeleted_ThenExpectRemoveOpAndNoObject",
			want:          jsonpatch.JSONopRemove,
			wantFinalizer: false,
			wantDeleted:   true,
			enabled:       true,
			instance: vshnv1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance-1",
				},
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "vshn-postgresql-instance-1",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{finalizerName},
				},
			},
		},
		{
			name:          "GivenNotEnabledAndNotDeleted_ThenExpectNoOpAndNoFinalizer",
			want:          jsonpatch.JSONopNone,
			wantFinalizer: false,
			enabled:       false,
			instance: vshnv1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance-1",
				},
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
		},
		{
			name:          "GivenNotEnabledAndNotFound_ThenExpectNoOpAndNoObject",
			want:          jsonpatch.JSONopNone,
			wantFinalizer: false,
			wantDeleted:   true,
			enabled:       false,
			instance: vshnv1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance-1",
				},
			},
			namespace: corev1.Namespace{},
		},
		{
			name:          "GivenNotEnabledAndFinalizer_ThenExpectRemoveOpAndNoFinalizer",
			want:          jsonpatch.JSONopRemove,
			wantFinalizer: false,
			enabled:       false,
			instance: vshnv1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name: "instance-1",
				},
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "vshn-postgresql-instance-1",
					Finalizers: []string{finalizerName},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Given
			fclient := fake.NewClientBuilder().
				WithScheme(s).
				WithRuntimeObjects(&tt.instance, &tt.namespace).
				Build()
			logger := logr.Discard()

			// When
			got, err := instanceNamespaceDeleted(context.TODO(), logger, &tt.instance, tt.enabled, fclient)
			if (err != nil) != tt.wantErr {
				t.Errorf("instanceNamespaceDeleted() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Then
			if got != tt.want {
				t.Errorf("instanceNamespaceDeleted() = %v, want %v", got, tt.want)
			}

			resultNs := &corev1.Namespace{}
			err = fclient.Get(context.TODO(), client.ObjectKey{Name: "vshn-postgresql-" + tt.instance.Name}, resultNs)

			if tt.wantDeleted {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantFinalizer {
				assert.Contains(t, resultNs.GetFinalizers(), finalizerName)
			} else {
				assert.NotContains(t, resultNs.GetFinalizers(), finalizerName)
			}
		})
	}
}
