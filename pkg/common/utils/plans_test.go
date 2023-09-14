package utils

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFetchPlansFromCluster(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
		Data: map[string]string{
			"plans": `
			{
				"plus-2": {
					"note": "Will be scheduled on APPUiO Cloud plus nodes",
					"scheduling": {
						"nodeSelector": {
							"appuio.io/node-class": "plus"
						}
					},
					"size": {
						"cpu": "400m",
						"disk": "20Gi",
						"enabled": true,
						"memory": "1728Mi"
					}
				},
				"plus-4": {
					"note": "Will be scheduledon APPUiO Cloud plus nodes",
					"scheduling": {
						"nodeSelector": {
							"appuio.io/node-class": "plus"
						}
					},
					"size": {
						"cpu": "900m",
						"disk": "40Gi",
						"enabled": true,
						"memory": "3776Mi"
					}
				},
				"standard-2": {
					"size": {
						"cpu": "400m",
						"disk": "20Gi",
						"enabled": true,
						"memory": "1728Mi"
					}
				},
				"standard-4": {
					"size": {
						"cpu": "900m",
						"disk": "40Gi",
						"enabled": true,
						"memory": "3776Mi"
					}
				}
			}
			`,
		},
	}

	type args struct {
		name string
		plan string
		ns   string
	}
	tests := []struct {
		name    string
		args    args
		want    Resources
		wantErr bool
	}{
		{
			name: "GivenCorrectNs_ThenGetCorrectPlan",
			args: args{
				name: "test",
				plan: "plus-2",
				ns:   "testns",
			},
			want: Resources{
				CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
				Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
			},
		},
		{
			name: "GivenWrongNS_ThenGetError",
			args: args{
				name: "test",
				plan: "plus-2",
				ns:   "wrong",
			},
			want:    Resources{},
			wantErr: true,
		},
		{
			name: "GivenWrongName_ThenGetError",
			args: args{
				name: "wrong",
				plan: "plus-2",
				ns:   "testns",
			},
			want:    Resources{},
			wantErr: true,
		},
		{
			name: "GivenWrongPlan_ThenGetError",
			args: args{
				name: "test",
				plan: "wrong",
				ns:   "testns",
			},
			want:    Resources{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := context.TODO()
			fclient := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).
				WithObjects(cm).Build()

			viper.Set("PLANS_NAMESPACE", tt.args.ns)
			viper.AutomaticEnv()

			got, err := FetchPlansFromCluster(ctx, fclient, tt.args.name, tt.args.plan)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assertEqualRessource(t, tt.want, got)
			}
		})
	}

}

func assertEqualRessource(t assert.TestingT, want, got Resources) {
	assert.Equal(t, want.CPULimits.MilliValue(), got.CPULimits.MilliValue())
	assert.Equal(t, want.CPURequests.MilliValue(), got.CPURequests.MilliValue())
	assert.Equal(t, want.MemoryLimits.Value(), got.MemoryLimits.Value())
	assert.Equal(t, want.MemoryRequests.Value(), got.MemoryRequests.Value())
	assert.Equal(t, want.Disk.Value(), got.Disk.Value())
}
