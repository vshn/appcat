package vshnnextcloud

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/utils/ptr"
)

func Test_addPostgreSQL(t *testing.T) {

	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/empty.yaml")

	comp := &vshnv1.VSHNNextcloud{}

	assert.NoError(t, addPostgreSQL(svc, comp))

	pg := &vshnv1.XVSHNPostgreSQL{}

	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

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

	assert.NoError(t, addPostgreSQL(svc, comp))
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))
	assert.False(t, *pg.Spec.Parameters.Backup.DeletionProtection)
	assert.Equal(t, 1, pg.Spec.Parameters.Backup.Retention)
}

func Test_addNextcloud(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, true, extDb["enabled"])
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, false, intDb["enabled"])

}

func Test_addReleaseInternalDB(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/02_no_pg.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, map[string]any{}, extDb)
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, true, intDb["enabled"])
}

func getNextcloudComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNNextcloud) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNNextcloud{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
