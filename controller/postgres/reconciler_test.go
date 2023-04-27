package postgres

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat-apiserver/controller/test/mocks"
	v1 "github.com/vshn/component-appcat/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
	"time"
)

func Test_Reconcile(t *testing.T) {
	previousDay := metav1.Time{Time: getCurrentTime().AddDate(0, 0, -1)}
	tests := map[string]struct {
		req                 reconcile.Request
		inst                v1.XVSHNPostgreSQL
		getInstanceTimes    int
		patchInstanceTimes  int
		deleteInstanceTimes int
		expectedResult      ctrl.Result
		expectedError       error
	}{
		"WhenInstanceNotDeletedAndNoFinalizer_ThenPatchAndDontDeleteInstanceAndRequeueDefault": {
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "instance-1",
					Name:      "namespace-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance-1",
					Namespace: "namespace-1",
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
						},
					},
				},
			},
			getInstanceTimes:    1,
			patchInstanceTimes:  1,
			deleteInstanceTimes: 0,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 30,
			},
		},
		"WhenInstanceNotDeletedAndFinalizer_ThenNoPatchAndDontDeleteInstanceAndRequeueDefault": {
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "instance-1",
					Name:      "namespace-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "instance-1",
					Namespace:  "namespace-1",
					Finalizers: []string{finalizerName},
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
						},
					},
				},
			},
			getInstanceTimes:    1,
			patchInstanceTimes:  0,
			deleteInstanceTimes: 0,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 30,
			},
		},
		"WhenInstanceDeletedAndFinalizer_ThenNoPatchAndDontDeleteInstanceAndRequeueDefault": {
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "instance-1",
					Name:      "namespace-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
					Namespace:         "namespace-1",
					DeletionTimestamp: &previousDay,
					Finalizers:        []string{finalizerName},
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
							DeletionRetention:  2,
						},
					},
				},
			},
			getInstanceTimes:    1,
			patchInstanceTimes:  0,
			deleteInstanceTimes: 1,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Hour * 24,
			},
		},
		"WhenInstanceDeletedAndRetentionHigherThanCurrentTime_ThenDeleteInstanceAndRequeueDifferenceTime": {
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "instance-1",
					Name:      "namespace-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
					Namespace:         "namespace-1",
					DeletionTimestamp: &previousDay,
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
							DeletionRetention:  2,
						},
					},
				},
			},
			getInstanceTimes:    1,
			patchInstanceTimes:  1,
			deleteInstanceTimes: 1,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Hour * 24,
			},
		},
		"WhenInstanceDeletedAndRetentionLowerThanCurrentTime_ThenDeleteInstanceAndRequeueDifferenceTimeNegative": {
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "instance-1",
					Name:      "namespace-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
					Namespace:         "namespace-1",
					DeletionTimestamp: &previousDay,
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
							DeletionRetention:  0,
						},
					},
				},
			},
			getInstanceTimes:    1,
			patchInstanceTimes:  1,
			deleteInstanceTimes: 1,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: -time.Hour * 24,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockedClient := mocks.NewMockClient(mockCtrl)
			reconciler := XPostgreSQLDeletionProtectionReconciler{
				Client: mockedClient,
			}

			mockedClient.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any()).
				SetArg(2, tc.inst).
				MaxTimes(tc.getInstanceTimes)

			mockedClient.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				MaxTimes(tc.patchInstanceTimes)

			mockedClient.EXPECT().
				Delete(gomock.Any(), gomock.Any()).
				MaxTimes(tc.deleteInstanceTimes)

			// WHEN
			result, err := reconciler.Reconcile(context.Background(), tc.req)

			// THEN
			if tc.expectedError != nil {
				assert.Error(t, tc.expectedError, err)
			}
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
