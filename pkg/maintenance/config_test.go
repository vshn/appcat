package maintenance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetMaintenanceConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		service        string
		configMap      *corev1.ConfigMap
		wantServiceDis bool
		wantAppcatDis  bool
		wantErr        bool
	}{
		{
			name:    "both disabled for keycloak",
			service: "keycloak",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maintenance-config",
					Namespace: "syn-appcat-control",
				},
				Data: map[string]string{
					"keycloak.disableServiceRelease": "true",
					"keycloak.disableAppcatRelease":  "true",
				},
			},
			wantServiceDis: true,
			wantAppcatDis:  true,
		},
		{
			name:    "nothing disabled for redis",
			service: "redis",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maintenance-config",
					Namespace: "syn-appcat-control",
				},
				Data: map[string]string{
					"redis.disableServiceRelease": "false",
					"redis.disableAppcatRelease":  "false",
				},
			},
			wantServiceDis: false,
			wantAppcatDis:  false,
		},
		{
			name:    "missing keys default to false",
			service: "mariadb",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "maintenance-config",
					Namespace: "syn-appcat-control",
				},
				Data: map[string]string{},
			},
			wantServiceDis: false,
			wantAppcatDis:  false,
		},
		{
			name:           "missing configmap returns an error, defaults false",
			service:        "forgejo",
			configMap:      nil,
			wantServiceDis: false,
			wantAppcatDis:  false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{}
			if tt.configMap != nil {
				objs = append(objs, tt.configMap)
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

			cfg, err := GetMaintenanceConfig(context.Background(), c, "maintenance-config", "syn-appcat-control", tt.service)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantServiceDis, cfg.DisableServiceRelease)
			assert.Equal(t, tt.wantAppcatDis, cfg.DisableAppcatRelease)
		})
	}
}
