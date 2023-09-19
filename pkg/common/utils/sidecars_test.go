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

func TestFetchSidecarsFromCluster(t *testing.T) {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
		Data: map[string]string{
			"sideCars": `
			{
				"clusterController": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "188Mi"
					}
				},
				"createBackup": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"envoy": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "64Mi"
					}
				},
				"pgbouncer": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "16m",
						"memory": "32Mi"
					}
				},
				"postgresUtil": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "4Mi"
					}
				},
				"prometheusPostgresExporter": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "32Mi"
					}
				},
				"runDbops": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"setDbopsResult": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				}
			}
			`,
		},
	}

	type args struct {
		name    string
		sideCar string
		ns      string
	}
	tests := []struct {
		name    string
		args    args
		want    Resources
		wantErr bool
	}{
		{
			name: "GivenWrongNs_ThenGetError",
			args: args{
				name:    "test",
				sideCar: "clusterController",
				ns:      "wrong",
			},
			want:    Resources{},
			wantErr: true,
		},
		{
			name: "GivenWrongName_ThenGetError",
			args: args{
				name:    "wrong",
				sideCar: "clusterController",
				ns:      "testns",
			},
			want:    Resources{},
			wantErr: true,
		},
		{
			name: "GivenWrongSideCar_ThenGetError",
			args: args{
				name:    "test",
				sideCar: "wrong",
				ns:      "testns",
			},
			want:    Resources{},
			wantErr: true,
		},
		{
			name: "GivenCorrectSideCar_ThenGetCorrectResources",
			args: args{
				name:    "test",
				sideCar: "clusterController",
				ns:      "testns",
			},
			want: Resources{
				CPURequests:    *resource.NewMilliQuantity(32, resource.DecimalSI),
				CPULimits:      *resource.NewMilliQuantity(600, resource.DecimalSI),
				Disk:           *resource.NewQuantity(0, resource.BinarySI),
				MemoryRequests: *resource.NewQuantity(197132288, resource.BinarySI),
				MemoryLimits:   *resource.NewQuantity(805306368, resource.BinarySI),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			fclient := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).
				WithObjects(cm).Build()
			viper.Set("PLANS_NAMESPACE", tt.args.ns)
			viper.AutomaticEnv()

			got, err := FetchSidecarFromCluster(ctx, fclient, tt.args.name, tt.args.sideCar)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assertEqualRessource(t, tt.want, got)
				s, err := FetchSidecarsFromCluster(ctx, fclient, "test")
				assert.NoError(t, err)
				r, err := GetAllSideCarsResources(s)
				assert.NoError(t, err)
				assert.Equal(t, r.CPULimits.MilliValue(), resource.NewMilliQuantity(4800, resource.DecimalSI).MilliValue())
				assert.Equal(t, r.CPURequests.MilliValue(), resource.NewMilliQuantity(850, resource.DecimalSI).MilliValue())
				assert.Equal(t, r.MemoryLimits.Value(), resource.NewQuantity(6442450944, resource.BinarySI).Value())
				assert.Equal(t, r.MemoryRequests.Value(), resource.NewQuantity(1140850688, resource.BinarySI).Value())
			}
		})
	}
}
