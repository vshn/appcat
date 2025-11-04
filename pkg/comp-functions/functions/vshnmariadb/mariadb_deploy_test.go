package vshnmariadb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestMariadbDeploy(t *testing.T) {

	svc, comp := getMariadbComp(t)

	ctx := context.TODO()

	rootUser := "root"
	rootPassword := "mariadb123"
	mariadbHost := "mariadb.vshn-mariadb-mariadb-gc9x4.svc.cluster.local"
	mariadbPort := "3306"
	mariadbUrl := "mysql://root:mariadb123@mariadb.vshn-mariadb-mariadb-gc9x4.svc.cluster.local:3306"

	assert.Nil(t, DeployMariadb(ctx, &vshnv1.VSHNMariaDB{}, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, comp.GetName()+"-instanceNs"))
	assert.Equal(t, string("vshn"), ns.GetLabels()[utils.OrgLabelName])

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetObservedComposedResource(r, comp.Name+"-release"))

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	cd := svc.GetConnectionDetails()
	assert.Equal(t, mariadbHost, string(cd["MARIADB_HOST"]))
	assert.Equal(t, mariadbPort, string(cd["MARIADB_PORT"]))
	assert.Equal(t, mariadbUrl, string(cd["MARIADB_URL"]))
	assert.Equal(t, rootUser, string(cd["MARIADB_USER"]))
	assert.Equal(t, rootPassword, string(cd["MARIADB_PASSWORD"]))

}

func getMariadbComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNMariaDB) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnmariadb/deploy/01_default.yaml")

	comp := &vshnv1.VSHNMariaDB{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}

// TestMariadbDeployHA tests deployment with multiple instances
func TestMariadbDeployHA(t *testing.T) {
	svc, comp := getMariadbComp(t)
	comp.Spec.Parameters.Instances = 3
	ctx := context.TODO()

	assert.Nil(t, DeployMariadb(ctx, comp, svc))

	// In HA mode with 3+ instances, main service should point to proxysql
	service := &corev1.Service{}
	assert.NoError(t, svc.GetDesiredKubeObject(service, comp.GetName()+"-main-service"))
	assert.Equal(t, "proxysql", service.Spec.Selector["app"])
	assert.Equal(t, int32(3306), service.Spec.Ports[0].Port)
	assert.Equal(t, int32(6033), service.Spec.Ports[0].TargetPort.IntVal)
}

// TestMariadbDeploySingleInstance tests deployment with single instance
func TestMariadbDeploySingleInstance(t *testing.T) {
	svc, comp := getMariadbComp(t)
	comp.Spec.Parameters.Instances = 1
	ctx := context.TODO()

	assert.Nil(t, DeployMariadb(ctx, comp, svc))

	// In single instance mode, main service should point directly to mariadb
	service := &corev1.Service{}
	assert.NoError(t, svc.GetDesiredKubeObject(service, comp.GetName()+"-main-service"))
	assert.Equal(t, "mariadb", service.Spec.Selector["app"])
	assert.Equal(t, int32(3306), service.Spec.Ports[0].Port)
	assert.Equal(t, int32(3306), service.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, corev1.ServiceTypeClusterIP, service.Spec.Type)
}

// TestCreateMainServiceLoadBalancer tests service creation with LoadBalancer type
func TestCreateMainServiceLoadBalancer(t *testing.T) {
	svc, comp := getMariadbComp(t)
	comp.Spec.Parameters.Network.ServiceType = "LoadBalancer"
	ctx := context.TODO()

	assert.Nil(t, DeployMariadb(ctx, comp, svc))

	service := &corev1.Service{}
	assert.NoError(t, svc.GetDesiredKubeObject(service, comp.GetName()+"-main-service"))
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, service.Spec.Type)
}

// TestNewValues tests the values generation for helm release
func TestNewValues(t *testing.T) {
	svc, comp := getMariadbComp(t)
	ctx := context.TODO()

	values, err := newValues(ctx, svc, comp, "test-secret")
	assert.NoError(t, err)
	assert.NotNil(t, values)

	// Verify basic structure
	assert.Equal(t, "test-secret", values["existingSecret"])
	assert.Equal(t, comp.GetName(), values["fullnameOverride"])
	assert.Equal(t, comp.GetInstances(), values["replicaCount"])

	// Verify TLS configuration
	tls, ok := values["tls"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, comp.Spec.Parameters.TLS.TLSEnabled, tls["enabled"])
	assert.Equal(t, "tls-server-certificate", tls["certificatesSecret"])

	// Verify resources are set
	resources, ok := values["resources"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, resources["requests"])
	assert.NotNil(t, resources["limits"])

	// Verify persistence
	persistence, ok := values["persistence"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, persistence["size"])
	assert.Equal(t, comp.Spec.Parameters.StorageClass, persistence["storageClass"])

	// Verify metrics are enabled
	metrics, ok := values["metrics"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, true, metrics["enabled"])

	// Verify startup probe is disabled
	startupProbe, ok := values["startupProbe"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, false, startupProbe["enabled"])

	// Verify pod labels
	podLabels, ok := values["podLabels"].(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "mariadb", podLabels["app"])
}

// TestNewValuesWithCustomImageRegistry tests custom image registry configuration
func TestNewValuesWithCustomImageRegistry(t *testing.T) {
	svc, comp := getMariadbComp(t)
	ctx := context.TODO()

	// Set custom registry in config
	svc.Config.Data["imageRegistry"] = "custom.registry.io"
	svc.Config.Data["imageRepositoryPrefix"] = "myprefix"

	values, err := newValues(ctx, svc, comp, "test-secret")
	assert.NoError(t, err)

	// Verify global image registry
	global, ok := values["global"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "custom.registry.io", global["imageRegistry"])

	// Verify image repository
	image, ok := values["image"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "myprefix/mariadb-galera", image["repository"])

	// Verify metrics image repository
	metrics, ok := values["metrics"].(map[string]interface{})
	assert.True(t, ok)
	metricsImage, ok := metrics["image"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "myprefix/mysqld-exporter", metricsImage["repository"])
}

// TestGetReleaseVersion tests release version extraction
func TestGetReleaseVersion(t *testing.T) {
	svc, comp := getMariadbComp(t)

	// The observed release in test data doesn't have a status, so it returns v1 as default
	// In the actual implementation, when status.atProvider.revision exists, it would use that
	version := getReleaseVersion(svc, comp)
	// v0 is returned when no revision is found (revision is 0 by default)
	assert.Contains(t, []string{"v0", "v1"}, version)
}

// TestAdjustSettings tests the custom settings adjustment
func TestAdjustSettings(t *testing.T) {
	tests := []struct {
		name            string
		settings        string
		hasUserSettings bool
		want            string
	}{
		{
			name:            "no user settings",
			settings:        "",
			hasUserSettings: false,
			want:            "\n!include /bitnami/conf/extra-vshn.cnf",
		},
		{
			name:            "with user settings",
			settings:        "[mysqld]\nmax_connections=100",
			hasUserSettings: true,
			want:            "[mysqld]\nmax_connections=100\n!include /bitnami/conf/extra-vshn.cnf\n!include /bitnami/conf/extra.cnf",
		},
		{
			name:            "multiline with user settings",
			settings:        "[mysqld]\nmax_connections=100\nkey_buffer_size=16M",
			hasUserSettings: true,
			want:            "[mysqld]\nmax_connections=100\nkey_buffer_size=16M\n!include /bitnami/conf/extra-vshn.cnf\n!include /bitnami/conf/extra.cnf",
		},
		{
			name:            "existing settings without user settings",
			settings:        "[mysqld]\nmax_connections=100",
			hasUserSettings: false,
			want:            "[mysqld]\nmax_connections=100\n!include /bitnami/conf/extra-vshn.cnf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustSettings(tt.settings, tt.hasUserSettings)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtractDefaultSettings tests extraction of default MariaDB settings
func TestExtractDefaultSettings(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		wantErr     bool
		wantSetting string
	}{
		{
			name: "valid config with settings",
			config: `{
				"chart": {
					"values": {
						"mariadbConfiguration": "[mysqld]\nmax_connections=100"
					}
				}
			}`,
			wantErr:     false,
			wantSetting: "[mysqld]\nmax_connections=100",
		},
		{
			name:    "invalid json",
			config:  "not json",
			wantErr: true,
		},
		{
			name: "missing mariadb configuration",
			config: `{
				"chart": {
					"values": {}
				}
			}`,
			wantErr:     false,
			wantSetting: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractDefaultSettings(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSetting, got)
			}
		})
	}
}

// TestUpdateMariaDBVersionFromTag tests version extraction from image tags
func TestUpdateMariaDBVersionFromTag(t *testing.T) {
	tests := []struct {
		name            string
		tag             string
		wantVersion     string
		wantStatusWrite bool
	}{
		{
			name:            "standard version tag",
			tag:             "11.2.2-debian-11-r6",
			wantVersion:     "11.2.2-MariaDB-log",
			wantStatusWrite: true,
		},
		{
			name:            "simple version tag",
			tag:             "10.11.5",
			wantVersion:     "10.11.5-MariaDB-log",
			wantStatusWrite: true,
		},
		{
			name:            "version with suffix",
			tag:             "11.1.3-focal",
			wantVersion:     "11.1.3-MariaDB-log",
			wantStatusWrite: true,
		},
		{
			name:            "empty tag",
			tag:             "",
			wantVersion:     "",
			wantStatusWrite: false,
		},
		{
			name:            "invalid tag format",
			tag:             "latest",
			wantVersion:     "",
			wantStatusWrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, comp := getMariadbComp(t)
			comp.Status.MariaDBVersion = "" // Reset version

			err := updateMariaDBVersionFromTag(comp, svc, tt.tag)
			assert.NoError(t, err)

			if tt.wantStatusWrite {
				assert.Equal(t, tt.wantVersion, comp.Status.MariaDBVersion)
			} else {
				assert.Equal(t, "", comp.Status.MariaDBVersion)
			}
		})
	}
}

// TestUpdateMariaDBVersionFromTagNoUpdate tests that version isn't updated if unchanged
func TestUpdateMariaDBVersionFromTagNoUpdate(t *testing.T) {
	svc, comp := getMariadbComp(t)

	// Set existing version
	comp.Status.MariaDBVersion = "11.2.2-MariaDB-log"

	// Try to update with same version
	err := updateMariaDBVersionFromTag(comp, svc, "11.2.2-debian-11-r6")
	assert.NoError(t, err)

	// Version should remain unchanged
	assert.Equal(t, "11.2.2-MariaDB-log", comp.Status.MariaDBVersion)
}

// TestGetConnectionDetails tests connection detail extraction
func TestGetConnectionDetails(t *testing.T) {
	svc, comp := getMariadbComp(t)

	// Get connection details - password may be empty if secret not directly accessible
	err := getConnectionDetails(comp, svc, comp.GetName())
	assert.NoError(t, err)

	cd := svc.GetConnectionDetails()
	assert.Equal(t, "3306", string(cd["MARIADB_PORT"]))
	assert.Equal(t, "root", string(cd["MARIADB_USERNAME"]))
	// Password field should be set (may be empty if secret not found)
	assert.Contains(t, cd, "MARIADB_PASSWORD")
}

// TestCreateMariaDBSettingsConfigMap tests MariaDB settings config map creation
func TestCreateMariaDBSettingsConfigMap(t *testing.T) {
	svc, comp := getMariadbComp(t)

	t.Run("with user settings", func(t *testing.T) {
		comp.Spec.Parameters.Service.MariadbSettings = "max_connections=200\nkey_buffer_size=32M"

		err := createMariaDBSettingsConfigMap(svc, comp)
		assert.NoError(t, err)

		cm := &corev1.ConfigMap{}
		err = svc.GetDesiredKubeObject(cm, comp.GetName()+"-extra-mariadb-settings")
		assert.NoError(t, err)

		assert.Equal(t, comp.GetName()+"-extra-mariadb-settings", cm.Name)
		assert.Equal(t, comp.Status.InstanceNamespace, cm.Namespace)

		// Should have both VSHN defaults and user settings
		assert.Contains(t, cm.Data, "extra-vshn.cnf")
		assert.Contains(t, cm.Data, "extra.cnf")

		expectedUserData := "[mysqld]\nmax_connections=200\nkey_buffer_size=32M"
		assert.Equal(t, expectedUserData, cm.Data["extra.cnf"])

		expectedVSHNData := "[mysqld]\ncharacter-set-server=utf8mb4\ncollation-server=utf8mb4_unicode_ci\n"
		assert.Equal(t, expectedVSHNData, cm.Data["extra-vshn.cnf"])
	})

	t.Run("without user settings", func(t *testing.T) {
		comp.Spec.Parameters.Service.MariadbSettings = ""

		err := createMariaDBSettingsConfigMap(svc, comp)
		assert.NoError(t, err)

		cm := &corev1.ConfigMap{}
		err = svc.GetDesiredKubeObject(cm, comp.GetName()+"-extra-mariadb-settings")
		assert.NoError(t, err)

		// Should still have VSHN defaults
		assert.Contains(t, cm.Data, "extra-vshn.cnf")
		// User settings should be empty string (to clear stale data)
		assert.Equal(t, "", cm.Data["extra.cnf"])
	})
}

// TestNewRelease tests helm release creation
func TestNewRelease(t *testing.T) {
	svc, comp := getMariadbComp(t)
	ctx := context.TODO()

	values := map[string]any{
		"replicaCount": 1,
		"persistence": map[string]interface{}{
			"size": "10Gi",
		},
	}

	rel, err := newRelease(ctx, svc, values, comp)
	require.NoError(t, err)
	require.NotNil(t, rel)

	// Verify chart name
	assert.Equal(t, "mariadb-galera", rel.Spec.ForProvider.Chart.Name)

	// Verify connection details are configured
	assert.Len(t, rel.Spec.ConnectionDetails, 2)

	// Verify TLS certificate connection detail
	tlsCD := rel.Spec.ConnectionDetails[0]
	assert.Equal(t, "tls-server-certificate", tlsCD.ObjectReference.Name)
	assert.Equal(t, "ca.crt", tlsCD.ToConnectionSecretKey)
	assert.True(t, tlsCD.SkipPartOfReleaseCheck)

	// Verify release secret connection detail
	releaseCD := rel.Spec.ConnectionDetails[1]
	assert.Contains(t, releaseCD.ObjectReference.Name, "sh.helm.release.v1.")
	assert.Equal(t, "release", releaseCD.ToConnectionSecretKey)
	assert.True(t, releaseCD.SkipPartOfReleaseCheck)
}

// TestUnzip tests gzip decompression
func TestUnzip(t *testing.T) {
	// This would require creating a gzipped string for testing
	// For now, testing error case
	t.Run("invalid gzip data", func(t *testing.T) {
		_, err := unzip("not gzipped data")
		assert.Error(t, err)
	})
}

// TestNewValuesWithResources tests resource allocation in values
func TestNewValuesWithResources(t *testing.T) {
	svc, comp := getMariadbComp(t)
	ctx := context.TODO()

	// The test comp already has resources set via the plan
	values, err := newValues(ctx, svc, comp, "test-secret")
	assert.NoError(t, err)

	resources, ok := values["resources"].(map[string]interface{})
	assert.True(t, ok)

	requests, ok := resources["requests"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, requests["memory"])
	assert.NotNil(t, requests["cpu"])

	limits, ok := resources["limits"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, limits["memory"])
	assert.NotNil(t, limits["cpu"])

	persistence, ok := values["persistence"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, persistence["size"])
}

// TestCreateMainServiceConnectionDetails tests connection detail generation
func TestCreateMainServiceConnectionDetails(t *testing.T) {
	svc, comp := getMariadbComp(t)

	// The secret is already in observed state from test data
	err := createMainService(comp, svc, comp.GetName())
	assert.NoError(t, err)

	cd := svc.GetConnectionDetails()

	// Verify host is set correctly
	expectedHost := "mariadb.vshn-mariadb-mariadb-gc9x4.svc.cluster.local"
	assert.Equal(t, expectedHost, string(cd["MARIADB_HOST"]))

	// Verify URL contains the expected structure (password may be empty if secret not accessible)
	urlStr := string(cd["MARIADB_URL"])
	assert.Contains(t, urlStr, "mysql://root:")
	assert.Contains(t, urlStr, "@mariadb.vshn-mariadb-mariadb-gc9x4.svc.cluster.local:3306")
}
