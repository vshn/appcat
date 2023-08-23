package quotas

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg"
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
				orgLabelName: "test",
			},
		},
	}

	gr := schema.GroupResource{Group: "test", Resource: "test"}
	gk := schema.GroupKind{Group: "test", Kind: "test"}

	tests := []struct {
		name        string
		wantErr     bool
		requested   Resources
		instanceNS  *corev1.Namespace
		dummyNs     int
		NSOverrides int
	}{
		{
			name: "GivenNoInstanceNamespace_WhenCheckingAgainstDefault_ThenNoError",
			requested: Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
		},
		{
			name: "GivenNotInstancenamespace_WhenCheckingAgainstDefault_ThenError",
			requested: Resources{
				CPURequests:    *resource.NewMilliQuantity(5000, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			wantErr: true,
		},
		{
			name: "GivenInstancenamespace_WhenCheckingAgainst_ThenNoError",
			requested: Resources{
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
						orgLabelName: "test",
					},
				},
			},
		},
		{
			name: "GivenInstancenamespace_WhenCheckingAgainst_ThenError",
			requested: Resources{
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
						orgLabelName: "test",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GivenTooManyNamespaces_ThenError",
			requested: Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
			wantErr: true,
			dummyNs: 30,
		},
		{
			name: "GivenNamespaceOverride_ThenNoError",
			requested: Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
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

			if tt.instanceNS != nil {
				AddInitalNamespaceQuotas(ns)
				assert.NoError(t, fclient.Create(ctx, tt.instanceNS))
			}

			for i := 0; i < tt.dummyNs; i++ {
				assert.NoError(t, fclient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("instancens-%d", i),
						Labels: map[string]string{
							orgLabelName: "test",
						},
					},
				}))
			}

			if tt.NSOverrides > 0 {
				assert.NoError(t, fclient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nsOverrideCMNamespace,
					},
				}))

				assert.NoError(t, fclient.Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nsOverrideCMPrefix + "test",
						Namespace: nsOverrideCMNamespace,
					},
					Data: map[string]string{
						overrideCMDataFieldName: strconv.Itoa(tt.NSOverrides),
					},
				}))
			}

			//when
			checker := NewQuotaChecker(fclient, "mytest", "myns", "testns", tt.requested, gr, gk, true)
			err := checker.CheckQuotas(ctx)

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
