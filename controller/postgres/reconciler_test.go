package postgres

import (
	"context"
	"testing"
	"time"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/component-appcat/apis/vshn/v1"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	s = runtime.NewScheme()
)

func init() {
	_ = vshnv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
}

func Test_Reconcile(t *testing.T) {
	previousDay := metav1.Time{Time: getCurrentTime().AddDate(0, 0, -1)}
	tests := []struct {
		name                    string
		req                     reconcile.Request
		inst                    v1.XVSHNPostgreSQL
		instanceNamespace       corev1.Namespace
		expectedResult          ctrl.Result
		expectedError           error
		expectFinalizer         bool
		expectInstanceNamespace bool
	}{
		{
			name: "WhenInstanceNotDeletedAndNoFinalizer_ThenPatchAndDontDeleteInstanceAndRequeueDefault",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "instance-1",
					Finalizers: []string{"dummy"}, // we can't jsonpatch an empty array...
				},
				Spec: v1.VSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: true,
						},
					},
				},
			},
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectFinalizer: true,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 30,
			},
			expectInstanceNamespace: true,
		},
		{
			name: "WhenInstanceNotDeletedAndFinalizer_ThenNoPatchAndDontDeleteInstanceAndRequeueDefault",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "instance-1",
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
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectFinalizer: true,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 30,
			},
			expectInstanceNamespace: true,
		},
		{
			name: "WhenInstanceDeletedAndFinalizer_ThenNoPatchAndDontDeleteInstanceAndRequeueDefault",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
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
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectFinalizer: true,
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Hour * 24,
			},
			expectInstanceNamespace: true,
		},
		{
			name: "WhenInstanceDeletedAndRetentionHigherThanCurrentTime_ThenDeleteInstanceAndRequeueDifferenceTime",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
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
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Hour * 24,
			},
			expectInstanceNamespace: true,
		},
		{
			name: "WhenInstanceDeletedAndRetentionLowerThanCurrentTime_ThenDeleteInstanceAndRequeueDifferenceTimeNegative",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "instance-1",
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
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: -time.Hour * 24,
			},
			expectInstanceNamespace: true,
		},
		{
			name: "WhenInstanceNamespaceIsDeleted_ThenExpectRemoveFinalizerFromInstance",
			req: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "instance-1",
				},
			},
			inst: v1.XVSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "instance-1",
					Finalizers: []string{finalizerName},
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
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "vshn-postgresql-instance-1",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{finalizerName},
				},
			},
			expectedResult: ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 30,
			},
			expectInstanceNamespace: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			// GIVEN
			fclient := fake.NewFakeClientWithScheme(s, &tc.inst, &tc.instanceNamespace)
			reconciler := XPostgreSQLDeletionProtectionReconciler{
				Client: fclient,
			}

			// WHEN
			result, err := reconciler.Reconcile(context.Background(), tc.req)

			// THEN
			if tc.expectedError != nil {
				assert.Error(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedResult, result)

			// Assert that the composite finalizers are as expected
			resultComposite := &vshnv1.XVSHNPostgreSQL{}
			getObjectToAssert(t, resultComposite, fclient, client.ObjectKeyFromObject(&tc.inst))

			// Assert that the namespace also has the finalizers
			resultNs := &corev1.Namespace{}
			if tc.expectInstanceNamespace {
				getObjectToAssert(t, resultNs, fclient, client.ObjectKeyFromObject(&tc.instanceNamespace))
			} else {
				assert.Error(t, fclient.Get(context.TODO(), client.ObjectKeyFromObject(&tc.instanceNamespace), resultNs))
			}

			if tc.expectFinalizer {
				assert.Contains(t, resultComposite.GetFinalizers(), finalizerName)
				assert.Contains(t, resultNs.GetFinalizers(), finalizerName)
			} else {
				assert.NotContains(t, resultComposite.GetFinalizers(), finalizerName)
			}
		})
	}
}

func getObjectToAssert(t assert.TestingT, obj client.Object, fclient client.Client, key client.ObjectKey) {
	err := fclient.Get(context.TODO(), key, obj)
	assert.NoError(t, err)
}
