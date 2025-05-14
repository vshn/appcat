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

	t.Run("GivenPlan_ExpectPlanResources", func(t *testing.T) {
		const (
			plan = "small"
			cpu  = "1"
			mem  = "4Gi"
			disk = "50Gi"
		)

		svc, comp, secretName := bootstrapTest(t)
		svc.Config.Data["defaultPlan"] = plan
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		values := getReleaseValues(t, *release)
		// We explect plan resources
		assert.Equal(t, cpu, values["resources"].(map[string]any)["limits"].(map[string]any)["cpu"])
		assert.Equal(t, mem, values["resources"].(map[string]any)["limits"].(map[string]any)["memory"])
		assert.Equal(t, disk, values["persistence"].(map[string]any)["size"])
	})

	t.Run("GivenPlanAndExplicitSizeObj_ExpectSizeObjValues", func(t *testing.T) {
		const (
			plan   = "large"
			cpu    = "2"
			memory = "1337Gi"
			disk   = "123Gi"
		)

		svc, comp, secretName := bootstrapTest(t)
		svc.Config.Data["defaultPlan"] = plan
		comp.Spec.Parameters.Size.CPU = cpu
		comp.Spec.Parameters.Size.Memory = memory
		comp.Spec.Parameters.Size.Disk = disk
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		values := getReleaseValues(t, *release)
		// We expect our own values instead of plan values
		assert.Equal(t, cpu, values["resources"].(map[string]any)["limits"].(map[string]any)["cpu"])
		assert.Equal(t, memory, values["resources"].(map[string]any)["limits"].(map[string]any)["memory"])
		assert.Equal(t, disk, values["persistence"].(map[string]any)["size"])
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

	o, _ := json.MarshalIndent(svc.Config, "", "  ")
	t.Logf("Loaded config: %s", o)

	comp := &vshnv1.VSHNForgejo{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{
		"username": "forgejo_admin",
	}))
	assert.NoError(t, err)

	return svc, comp, secretName
}
