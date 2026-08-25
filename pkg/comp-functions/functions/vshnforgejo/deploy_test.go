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

	t.Run("GivenOAuth2ClientSettings_ExpectInHelmValues", func(t *testing.T) {
		svc, comp, secretName := bootstrapTest(t)
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))

		values := getReleaseValues(t, *release)
		config := values["gitea"].(map[string]any)["config"].(map[string]any)
		assert.Equal(t, map[string]any{"ENABLE_AUTO_REGISTRATION": "true"}, config["oauth2_client"])
		assert.Equal(t, map[string]any{"ENABLE": "true"}, config["oauth2"])
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

func TestForgejoConfigMerge(t *testing.T) {
	// configFromRelease runs addForgejo with the given user config and returns
	// the composed gitea.config map.
	configFromRelease := func(t *testing.T, cfg vshnv1.VSHNForgejoConfig) (map[string]any, string) {
		svc, comp, secretName := bootstrapTest(t)
		comp.Spec.Parameters.Service.ForgejoSettings.Config = cfg
		require := assert.New(t)
		require.NoError(addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		require.NoError(svc.GetDesiredComposedResourceByName(release, comp.GetName()))
		values := getReleaseValues(t, *release)
		return values["gitea"].(map[string]any)["config"].(map[string]any), comp.Spec.Parameters.Service.FQDN[0]
	}

	t.Run("EmptyServerMap_DefaultsIntact", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{Server: map[string]string{}})
		server := config["server"].(map[string]any)
		assert.Equal(t, fqdn, server["DOMAIN"])
		assert.Equal(t, "https://"+fqdn, server["ROOT_URL"])
		assert.Equal(t, true, server["DISABLE_SSH"])
		assert.Equal(t, "login", server["LANDING_PAGE"])
		assert.Equal(t, true, server["OFFLINE_MODE"])
	})

	t.Run("OverrideNonLockedDefault_UserWins", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{Server: map[string]string{
			"OFFLINE_MODE": "false",
			"LANDING_PAGE": "explore",
		}})
		server := config["server"].(map[string]any)
		assert.Equal(t, "false", server["OFFLINE_MODE"])
		assert.Equal(t, "explore", server["LANDING_PAGE"])
	})

	t.Run("OverrideLockedKeys_LockWins", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{Server: map[string]string{
			"DOMAIN":      "evil.example.com",
			"ROOT_URL":    "http://evil.example.com",
			"DISABLE_SSH": "false",
		}})
		server := config["server"].(map[string]any)
		assert.Equal(t, fqdn, server["DOMAIN"])
		assert.Equal(t, "https://"+fqdn, server["ROOT_URL"])
		assert.Equal(t, true, server["DISABLE_SSH"])
	})

	t.Run("AddNewServerKeys_Additive", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{Server: map[string]string{
			"CUSTOM_KEY":      "x",
			"SSH_LISTEN_PORT": "9999",
		}})
		server := config["server"].(map[string]any)
		assert.Equal(t, "x", server["CUSTOM_KEY"])
		assert.Equal(t, "9999", server["SSH_LISTEN_PORT"])
		// unrelated keys do not disturb the defaults
		assert.Equal(t, fqdn, server["DOMAIN"])
		assert.Equal(t, true, server["OFFLINE_MODE"])
	})

	t.Run("RepositoryMerge_RootDefaultPreserved", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{Repository: map[string]string{
			"DEFAULT_PRIVATE": "true",
		}})
		repo := config["repository"].(map[string]any)
		assert.Equal(t, "/data/git/repositories", repo["ROOT"])
		assert.Equal(t, "true", repo["DEFAULT_PRIVATE"])
	})

	t.Run("AdminMerge_DefaultPreserved", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{Admin: map[string]string{
			"DEFAULT_EMAIL_NOTIFICATIONS": "onmention",
		}})
		admin := config["admin"].(map[string]any)
		assert.Equal(t, true, admin["SEND_NOTIFICATION_EMAIL_ON_NEW_USER"])
		assert.Equal(t, "onmention", admin["DEFAULT_EMAIL_NOTIFICATIONS"])
	})

	t.Run("UnrelatedSectionWithoutDefault_SetWholesale", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{Mailer: map[string]string{
			"PROTOCOL":  "smtp",
			"SMTP_ADDR": "mail.example.com",
		}})
		mailer := config["mailer"].(map[string]any)
		assert.Equal(t, "smtp", mailer["PROTOCOL"])
		assert.Equal(t, "mail.example.com", mailer["SMTP_ADDR"])
	})
}

func TestDeploymentHTTPRoute(t *testing.T) {
	t.Run("GivenHTTPRouteMode_ExpectHTTPRouteAndListenerSet", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshnforgejo/03_httproute.yaml")
		svc.Config.Data["routeType"] = common.RouteTypeHTTPRoute
		svc.Config.Data["httpGatewayName"] = "http-gateway"
		svc.Config.Data["httpGatewayNamespace"] = "syn-kgateway"

		comp := &vshnv1.VSHNForgejo{}
		err := svc.GetObservedComposite(comp)
		assert.NoError(t, err)

		secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{
			"username": "forgejo_admin",
		}))
		assert.NoError(t, err)
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		allDesired := svc.GetAllDesired()
		foundRoute, foundLS, foundGrant := false, false, false
		for _, d := range allDesired {
			name := d.Resource.GetName()
			if name == comp.GetName()+"-httproute" {
				foundRoute = true
			}
			if name == comp.GetName()+"-listenerset" {
				foundLS = true
			}
			if name == comp.GetName()+"-httpgrant" {
				foundGrant = true
			}
		}
		assert.True(t, foundRoute)
		assert.True(t, foundLS)
		assert.False(t, foundGrant)
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
