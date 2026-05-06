package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/maintenance"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestShouldDisable(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cmName := "maintenance-config"
	cmNamespace := "syn-appcat-control"

	tests := []struct {
		name                    string
		service                 string
		envDisableAppcatRelease string
		cmData                  map[string]string
		wantDisableAppcat       bool
		wantDisableService      bool
	}{
		{
			name:                    "env disables appcat, cm does not",
			service:                 "keycloak",
			envDisableAppcatRelease: "true",
			cmData:                  map[string]string{},
			wantDisableAppcat:       true,
			wantDisableService:      false,
		},
		{
			name:                    "cm disables both, env does not",
			service:                 "redis",
			envDisableAppcatRelease: "false",
			cmData: map[string]string{
				"redis.disableServiceRelease": "true",
				"redis.disableAppcatRelease":  "true",
			},
			wantDisableAppcat:  true,
			wantDisableService: true,
		},
		{
			name:                    "both sources disable appcat",
			service:                 "mariadb",
			envDisableAppcatRelease: "true",
			cmData: map[string]string{
				"mariadb.disableAppcatRelease": "true",
			},
			wantDisableAppcat:  true,
			wantDisableService: false,
		},
		{
			name:                    "nothing disabled",
			service:                 "nextcloud",
			envDisableAppcatRelease: "false",
			cmData:                  map[string]string{},
			wantDisableAppcat:       false,
			wantDisableService:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: cmNamespace,
				},
				Data: tt.cmData,
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cm).Build()

			cfg, err := maintenance.GetMaintenanceConfig(context.Background(), c, cmName, cmNamespace, tt.service)
			assert.NoError(t, err)

			envDisable := tt.envDisableAppcatRelease == "true"

			disableAppcat := envDisable || cfg.DisableAppcatRelease
			disableService := cfg.DisableServiceMaint

			assert.Equal(t, tt.wantDisableAppcat, disableAppcat)
			assert.Equal(t, tt.wantDisableService, disableService)
		})
	}
}
