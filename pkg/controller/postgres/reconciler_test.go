package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	s = runtime.NewScheme()
)

func init() {
	_ = v1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
}

func Test_Reconcile(t *testing.T) {
	tests := []struct {
		name                    string
		req                     reconcile.Request
		inst                    v1.XVSHNPostgreSQL
		instanceNamespace       corev1.Namespace
		expectedResult          ctrl.Result
		expectedError           error
		expectInstanceNamespace bool
	}{
		{
			name: "WhenFinalizer_ThenPatchInstance",
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
				Spec: v1.XVSHNPostgreSQLSpec{
					Parameters: v1.VSHNPostgreSQLParameters{
						Backup: v1.VSHNPostgreSQLBackup{
							DeletionProtection: ptr.To(true),
						},
					},
				},
			},
			instanceNamespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vshn-postgresql-instance-1",
				},
			},
			expectedResult:          ctrl.Result{},
			expectInstanceNamespace: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			// GIVEN
			fclient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(&tc.inst, &tc.instanceNamespace).
				Build()
			reconciler := XPostgreSQLReconciler{
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
			resultComposite := &v1.XVSHNPostgreSQL{}
			getObjectToAssert(t, resultComposite, fclient, client.ObjectKeyFromObject(&tc.inst))

			// Assert that the namespace also has the finalizers
			resultNs := &corev1.Namespace{}
			if tc.expectInstanceNamespace {
				getObjectToAssert(t, resultNs, fclient, client.ObjectKeyFromObject(&tc.instanceNamespace))
			} else {
				assert.Error(t, fclient.Get(context.TODO(), client.ObjectKeyFromObject(&tc.instanceNamespace), resultNs))
			}

			assert.NotContains(t, resultComposite.GetFinalizers(), finalizerName)
		})
	}
}

func getObjectToAssert(t assert.TestingT, obj client.Object, fclient client.Client, key client.ObjectKey) {
	err := fclient.Get(context.TODO(), key, obj)
	assert.NoError(t, err)
}
