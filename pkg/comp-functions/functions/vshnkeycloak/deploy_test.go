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

	assert.NoError(t, common.NewPostgreSQLDependencyBuilder(svc, comp).
		AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
		CreateDependency())

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

	assert.NoError(t, common.NewPostgreSQLDependencyBuilder(svc, comp).AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).CreateDependency())
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

	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret"))

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

	assert.NoError(t, addRelease(context.TODO(), svc, comp, "mysecret"))
	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))
	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	assert.Equal(t, float64(2), values["replicas"])

	pg := &vshnv1.XVSHNPostgreSQL{}

	assert.NoError(t, common.NewPostgreSQLDependencyBuilder(svc, comp).AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).CreateDependency())
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+common.PgInstanceNameSuffix))
	assert.Equal(t, 2, pg.Spec.Parameters.Instances)

}
