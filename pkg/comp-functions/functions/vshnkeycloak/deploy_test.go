package vshnkeycloak

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func Test_addPostgreSQL(t *testing.T) {

	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/empty.yaml")

	comp := &vshnv1.VSHNKeycloak{}

	_, err := common.NewPostgreSQLDependencyBuilder(svc, comp).
		AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
		CreateDependency()
	assert.NoError(t, err)

	pg := &vshnv1.XVSHNPostgreSQL{}

	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+common.PgInstanceNameSuffix))

	// Assert default values
	assert.True(t, *pg.Spec.Parameters.Backup.DeletionProtection)
	assert.Equal(t, 6, pg.Spec.Parameters.Backup.Retention)

	// Assert default overrides
	comp.Spec.Parameters.Service.PostgreSQLParameters = &vshnv1.VSHNPostgreSQLParameters{
		Backup: vshnv1.VSHNPostgreSQLBackup{
			DeletionProtection: ptr.To(false),
			Retention:          1,
		},
	}

	_, err = common.NewPostgreSQLDependencyBuilder(svc, comp).AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).CreateDependency()
	assert.NoError(t, err)
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+common.PgInstanceNameSuffix))
	assert.False(t, *pg.Spec.Parameters.Backup.DeletionProtection)
	assert.Equal(t, 1, pg.Spec.Parameters.Backup.Retention)
}

func Test_addRelease(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/01_default.yaml")

	comp := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycloak",
			Namespace: "default",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					Version: "23",
				},
			},
		},
	}

	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret", "mysecret"))

	release := &xhelmv1.Release{}

	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

}

func Test_addHARelease(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/01_default.yaml")

	comp := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycloak",
			Namespace: "default",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Instances: 2,
				Service: vshnv1.VSHNKeycloakServiceSpec{
					Version: "23",
				},
			},
		},
	}

	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret", "mysecret"))
	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))
	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	assert.Equal(t, float64(2), values["replicas"])

	pg := &vshnv1.XVSHNPostgreSQL{}

	_, err := common.NewPostgreSQLDependencyBuilder(svc, comp).AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).CreateDependency()
	assert.NoError(t, err)
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+common.PgInstanceNameSuffix))
	assert.Equal(t, 2, pg.Spec.Parameters.Instances)
}

func Test_addCustomEnvSecrets(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/01_default.yaml")

	comp := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mycloak",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Instances: 2,
				Service: vshnv1.VSHNKeycloakServiceSpec{
					Version:               "23",
					CustomEnvVariablesRef: ptr.To("my-secret-1"), // This is a deprecated field, but still needs to work for backwards compatibility
					EnvFrom: &[]corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-configmap",
								},
							},
						},
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-secret-2",
								},
							},
						},
					},
				},
			},
		},
	}

	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret", "mysecret"))
	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))

	var envFrom []corev1.EnvFromSource
	rawYaml := values["extraEnvFrom"].(string)
	assert.NoError(t, yaml.Unmarshal([]byte(rawYaml), &envFrom))

	assert.Equal(t, 3, len(envFrom))
	for _, env := range envFrom {
		if env.ConfigMapRef != nil {
			assert.Equal(t, "my-configmap", env.ConfigMapRef.Name)
		} else if env.SecretRef != nil {
			assert.True(t, env.SecretRef.Name == "my-secret-1" || env.SecretRef.Name == "my-secret-2")
		}
	}
}

func Test_addCustomFiles(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/01_default.yaml")
	comp := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycloak",
			Namespace: "default",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					Version: "23",
					CustomizationImage: vshnv1.VSHNKeycloakCustomizationImage{
						Image: "dummy-image:latest",
					},
					CustomFiles: []vshnv1.VSHNKeycloakCustomFile{},
				},
			},
		},
	}

	// Valid path
	t.Log("Testing valid path")
	comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{
		{
			Source:      "folder1",
			Destination: "folder1",
		},
		{
			Source:      "blacklist.txt",
			Destination: "data/password-blacklists/list.txt",
		},
	}

	_, err := addCustomFileCopyInitContainer(comp, []map[string]any{})
	assert.NoError(t, err)

	// Are values good?
	t.Log("Checking values")
	values, err := newValues(context.TODO(), svc, comp, "a", "b")
	assert.NoError(t, err)

	volumeMounts := []map[string]any{}
	err = yaml.Unmarshal([]byte(values["extraVolumeMounts"].(string)), &volumeMounts)
	assert.NoError(t, err)

	foundCustomFileMount := false
	for _, volumeMount := range volumeMounts {
		if volumeMount["name"] == "customization-image-files" {
			foundCustomFileMount = true

			p := volumeMount["mountPath"]
			s := volumeMount["subPath"]
			assert.True(t, p == "/opt/keycloak/folder1" || p == "/opt/keycloak/data/password-blacklists/list.txt")
			assert.True(t, s == "folder1" || s == "data/password-blacklists/list.txt")
		}
	}

	assert.True(t, foundCustomFileMount)

	// Invalid paths
	for _, folder := range []string{
		"providers",
		"themes",
		"lib",
		"conf",
		"bin",
	} {
		t.Log("Testing invalid destination: ", folder)
		comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      folder,
				Destination: folder,
			},
		}

		_, err := addCustomFileCopyInitContainer(comp, []map[string]any{})
		assert.Error(t, err)
	}

	for _, destination := range []string{
		"../../etc/passwd",
		"./../../etc/passwd",
		"data/../../../etc/passwd",
		"./data/../../../etc/passwd",
		"/data/../../../etc/passwd",
	} {
		t.Log("Testing path traversal: ", destination)
		comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "file.txt",
				Destination: destination,
			},
		}
		_, err = addCustomFileCopyInitContainer(comp, []map[string]any{})
		assert.Error(t, err)
	}

	// No custom files added
	t.Log("Testing no custom files")
	comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{}
	comp.Spec.Parameters.Service.CustomizationImage = vshnv1.VSHNKeycloakCustomizationImage{}
	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret", "mysecret"))
	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))
	values = map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))

	v, exists := values["extraInitContainers"].(string)
	assert.True(t, exists, "extraInitContainers should exist in values")
	assert.NotEqual(t, "null\n", v, "extraInitContainers should not be 'null\\n' when no custom files are added")
}

func Test_configOrEnvChanged(t *testing.T) {
	configHash1 := "0bee89b07a248e27c83fc3d5951213c1"
	configHash2 := "b6273b589df2dfdbd8fe35b1011e3183"
	envHash1 := "614dd0e977becb4c6f7fa99e64549b12"
	envHash2 := "eadcf91a0d9dd70ad5da26a885f4c10c"

	tests := []struct {
		name              string
		currentConfigHash string
		lastConfigHash    string
		currentEnvHash    string
		lastEnvHash       string
		want              bool
	}{
		{
			name:              "no change",
			currentConfigHash: configHash1,
			lastConfigHash:    configHash1,
			currentEnvHash:    envHash1,
			lastEnvHash:       envHash1,
			want:              false,
		},
		{
			name:              "config changed",
			currentConfigHash: configHash1,
			lastConfigHash:    configHash2,
			currentEnvHash:    envHash1,
			lastEnvHash:       envHash1,
			want:              true,
		},
		{
			name:              "env changed",
			currentConfigHash: configHash1,
			lastConfigHash:    configHash1,
			currentEnvHash:    envHash1,
			lastEnvHash:       envHash2,
			want:              true,
		},
		{
			name:              "both changed",
			currentConfigHash: configHash1,
			lastConfigHash:    configHash2,
			currentEnvHash:    envHash1,
			lastEnvHash:       envHash2,
			want:              true,
		},
		{
			name:              "empty config hash",
			currentConfigHash: "",
			lastConfigHash:    configHash2,
			currentEnvHash:    envHash1,
			lastEnvHash:       envHash1,
			want:              false,
		},
		{
			name:              "empty env hash",
			currentConfigHash: configHash1,
			lastConfigHash:    configHash1,
			currentEnvHash:    "",
			lastEnvHash:       envHash2,
			want:              false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configOrEnvChanged(tt.currentConfigHash, tt.lastConfigHash, tt.currentEnvHash, tt.lastEnvHash)
			assert.Equal(t, tt.want, got)
		})
	}
}
