package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
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
		obj        vshnv1.XVSHNPostgreSQL
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
		obj              vshnv1.XVSHNPostgreSQL
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
		obj           vshnv1.XVSHNPostgreSQL
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

func transformToK8sTime(t *time.Time) *metav1.Time {
	if t != nil {
		temp := metav1.NewTime(*t)
		return &temp
	}
	return nil
}

func getXVSHNPostgreSQL(addFinalizer bool, deletedTime *time.Time) vshnv1.XVSHNPostgreSQL {
	obj := vshnv1.XVSHNPostgreSQL{}
	if addFinalizer {
		obj.Finalizers = []string{finalizerName}
	}
	if deletedTime != nil {
		obj.SetDeletionTimestamp(&metav1.Time{Time: *deletedTime})
	}
	return obj
}

func getPatch(op jsonOp) client.Patch {
	strIndex := strconv.Itoa(0)
	if op == opAdd {
		strIndex = "-"
	}
	patchOps := []jsonpatch{
		{
			Op:    op,
			Path:  "/metadata/finalizers/" + strIndex,
			Value: finalizerName,
		},
	}
	patch, _ := json.Marshal(patchOps)
	return client.RawPatch(types.JSONPatchType, patch)
}

func Test_instanceNamespaceDeleted(t *testing.T) {
	tests := []struct {
		name          string
		want          jsonOp
		wantDeleted   bool
		wantFinalizer bool
		wantErr       bool
		instance      vshnv1.XVSHNPostgreSQL
		namespace     corev1.Namespace
		enabled       bool
	}{
		{
			name:          "GivenEnabledAndNotDeleted_ThenExpectNoOpAndFinalizer",
			want:          opNone,
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
			want:          opRemove,
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
			want:          opNone,
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
			want:          opNone,
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
			want:          opRemove,
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
