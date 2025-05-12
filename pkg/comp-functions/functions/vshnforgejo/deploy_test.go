package vshnforgejo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func TestDeployment(t *testing.T) {
	t.Run("GivenNoFQDN_ExpectError", func(t *testing.T) {
		svc, comp, secretName := bootstrapTest(t)
		comp.Spec.Parameters.Service.FQDN = []string{}
		assert.Error(t, addForgejo(context.TODO(), svc, comp, secretName))
	})

	t.Run("GivenNoServiceVersion_ExpectError", func(t *testing.T) {
		svc, comp, secretName := bootstrapTest(t)
		comp.Spec.Parameters.Service.MajorVersion = ""
		assert.Error(t, addForgejo(context.TODO(), svc, comp, secretName))
	})

	t.Run("Test_addForgejo", func(t *testing.T) {
		svc, comp, secretName := bootstrapTest(t)
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		getReleaseValues(t, *release)
	})

	t.Run("Ensure_AppNameCanBeDefined", func(t *testing.T) {
		const appName = "My_App"

		svc, comp, secretName := bootstrapTest(t)
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		values := getReleaseValues(t, *release)
		assert.Equal(t, appName, values["gitea"].(map[string]any)["config"].(map[string]any)["APP_NAME"])
	})

	t.Run("Ensure_DiskSizeCanBeSet", func(t *testing.T) {
		const diskSize = "15Gi"

		svc, comp, secretName := bootstrapTest(t)
		comp.Spec.Parameters.Size.Disk = diskSize
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		values := getReleaseValues(t, *release)
		assert.Equal(t, diskSize, values["persistence"].(map[string]any)["size"])
	})
}

func getReleaseValues(t *testing.T, release xhelmv1.Release) map[string]any {
	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	assert.Greater(t, len(values), 0)

	return values
}

func bootstrapTest(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNForgejo, string) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnforgejo/01_default.yaml")

	comp := &vshnv1.VSHNForgejo{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{
		"username": "forgejo_admin",
	}))
	assert.NoError(t, err)

	return svc, comp, secretName
}
