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
	values, err := newValues(context.TODO(), svc, comp, "a", "b", "c")
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
}
