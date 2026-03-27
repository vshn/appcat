package quotas

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/apis/metadata"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestQuotaChecker_CheckQuotas(t *testing.T) {
	ctx := context.TODO()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myns",
			Labels: map[string]string{
				utils.OrgLabelName: "test",
			},
		},
	}

	gr := schema.GroupResource{Group: "test", Resource: "test"}
	gk := schema.GroupKind{Group: "test", Kind: "test"}

	tests := []struct {
		name        string
		wantErr     bool
		requested   utils.Resources
		instanceNS  *corev1.Namespace
		dummyNs     int
		NSOverrides int
		instances   int64
	}{
		{
			name: "GivenNoInstanceNamespace_WhenCheckingAgainstDefault_ThenNoError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instances: 1,
		},
		{
			name: "GivenNotInstancenamespace_WhenCheckingAgainstDefault_ThenError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(5000, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instances: 1,
			wantErr:   true,
		},
		{
			name: "GivenInstancenamespace_WhenCheckingAgainst_ThenNoError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instanceNS: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testns",
					Labels: map[string]string{
						utils.OrgLabelName: "test",
					},
				},
			},
			instances: 1,
		},
		{
			name: "GivenInstancenamespace_WhenCheckingAgainst_ThenError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(5000, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instanceNS: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testns",
					Labels: map[string]string{
						utils.OrgLabelName: "test",
					},
				},
			},
			instances: 1,
			wantErr:   true,
		},
		{
			name: "GivenTooManyNamespaces_ThenError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instances: 1,
			wantErr:   true,
			dummyNs:   30,
		},
		{
			name: "GivenNamespaceOverride_ThenNoError",
			requested: utils.Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			instances:   1,
			dummyNs:     30,
			NSOverrides: 35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			fclient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(ns).Build()

			svc := commontest.LoadRuntimeFromFile(t, "common/quotas/01_default.yaml")
			objectMeta := &metadata.MetadataOnlyObject{}

			err := svc.GetObservedComposite(objectMeta)
			assert.NoError(t, err)

			if tt.instanceNS != nil {
				s := &utils.Sidecars{}
				assert.NoError(t, err)
				AddInitalNamespaceQuotas(ctx, ns, s, gk.Kind, "")
				assert.NoError(t, fclient.Create(ctx, tt.instanceNS))
			}

			for i := 0; i < tt.dummyNs; i++ {
				assert.NoError(t, fclient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("instancens-%d", i),
						Labels: map[string]string{
							utils.OrgLabelName: "test",
						},
					},
				}))
			}

			if tt.NSOverrides > 0 {
				assert.NoError(t, fclient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: utils.NsOverrideCMNamespace,
					},
				}))

				assert.NoError(t, fclient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.NsOverrideCMPrefix + "test",
						Namespace: utils.NsOverrideCMNamespace,
					},
					Data: map[string]string{
						utils.OverrideCMDataFieldName: strconv.Itoa(tt.NSOverrides),
					},
				}))
			}

			//when
			checker := NewQuotaChecker(fclient, "mytest", "myns", "testns", tt.requested, gr, gk, true, tt.instances)
			err = checker.CheckQuotas(ctx)

			//Then
			// Unfortunately assert.Error and assert.NoError give us false positives here.
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}

}
