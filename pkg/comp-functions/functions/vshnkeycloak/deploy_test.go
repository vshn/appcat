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
					Version:     "23",
					CustomFiles: []vshnv1.VSHNKeycloakCustomFile{},
				},
			},
		},
	}

	// Valid path
	comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{
		{
			Source:      "folder1",
			Destination: "folder1",
		},
	}

	extraVolumesMap, extraVolumeMountsMap := addCustomFilesMounts(comp, svc)
	o1, _ := json.MarshalIndent(extraVolumesMap, "", "  ")
	o2, _ := json.MarshalIndent(extraVolumeMountsMap, "", "  ")
	t.Log("Extra Volumes: ", string(o1), "Extra Volume Mounts: ", string(o2))

	const expectedName = "custom-file-folder1"
	assert.Equal(t, 1, len(extraVolumesMap))
	assert.Equal(t, expectedName, extraVolumesMap[0]["name"])
	assert.Equal(t, expectedName, extraVolumeMountsMap[0]["name"])
	assert.Equal(t, "/opt/keycloak/folder1", extraVolumeMountsMap[0]["mountPath"])

	// Invalid paths
	for _, folder := range []string{
		"providers",
		"themes",
		"lib",
		"data",
		"conf",
		"bin",
	} {
		comp.Spec.Parameters.Service.CustomFiles = []vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "folder1",
				Destination: folder,
			},
		}

		extraVolumesMap, extraVolumeMountsMap := addCustomFilesMounts(comp, svc)
		assert.Equal(t, 0, len(extraVolumesMap))
		assert.Equal(t, 0, len(extraVolumeMountsMap))
	}
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
