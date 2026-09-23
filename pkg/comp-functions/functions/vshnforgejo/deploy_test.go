package vshnforgejo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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

	t.Run("OverrideNonLockedDefaultAndAddKeys_UserWins", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{Server: map[string]string{
			"OFFLINE_MODE":    "false",
			"LANDING_PAGE":    "explore",
			"CUSTOM_KEY":      "x",
			"SSH_LISTEN_PORT": "9999",
		}})
		server := config["server"].(map[string]any)
		assert.Equal(t, "false", server["OFFLINE_MODE"])
		assert.Equal(t, "explore", server["LANDING_PAGE"])
		assert.Equal(t, "x", server["CUSTOM_KEY"])
		assert.Equal(t, "9999", server["SSH_LISTEN_PORT"])
		assert.Equal(t, fqdn, server["DOMAIN"])
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

	t.Run("CronArchiveCleanupOverride_DefaultScheduleKept", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{CronArchiveCleanup: map[string]string{
			"OLDER_THAN": "72h",
		}})
		archive := config["cron.archive_cleanup"].(map[string]any)
		assert.Equal(t, "72h", archive["OLDER_THAN"])
		assert.Equal(t, "@hourly", archive["SCHEDULE"])
	})

	t.Run("CronGitGCRepos_SetWholesale", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{CronGitGCRepos: map[string]string{
			"SCHEDULE": "@every 72h",
			"TIMEOUT":  "120s",
		}})
		gc := config["cron.git_gc_repos"].(map[string]any)
		assert.Equal(t, "@every 72h", gc["SCHEDULE"])
		assert.Equal(t, "120s", gc["TIMEOUT"])
	})

	t.Run("ManagedKeys_ExactAndAliasSpellingsDropped", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{
			Server: map[string]string{
				"domain": "evil.example.com", " ROOT_URL ": "http://evil.example.com", "disable_ssh": "false",
			},
			Security: map[string]string{
				"disable_git_hooks": "false", "Import_Local_Paths": "true", " secret_key_uri ": "file:/tmp/secret",
				"DISABLE_GIT_HOOKS__FILE": "/tmp/false", "_0X44495341424C455F4749545F484F4F4B53_": "false",
				"\"IMPORT_LOCAL_PATHS\"":            "true",
				"reverse_proxy_trusted_proxies":     "1.2.3.4",
				"SECRET_KEY":                        "hijack",
				"REVERSE_PROXY_LIMIT":               "5",
				"REVERSE_PROXY_AUTHENTICATION_USER": "true",
				"INSTALL_LOCK":                      "false",
				"MIN_PASSWORD_LENGTH":               "12",
			},
			Picture:    map[string]string{"avatar_upload_path": "/tmp/avatars", "AVATAR_MAX_WIDTH": "1024"},
			Repository: map[string]string{"root": "/tmp/repos", "ROOT": "/tmp/repos", "DEFAULT_PRIVATE": "true"},
			GitConfig:  map[string]string{"remote.MyRemote.url": "https://example.com/repo.git"},
		})

		server := config["server"].(map[string]any)
		assert.Equal(t, fqdn, server["DOMAIN"])
		assert.Equal(t, "https://"+fqdn, server["ROOT_URL"])
		assert.Equal(t, true, server["DISABLE_SSH"])
		for _, key := range []string{"domain", " ROOT_URL ", "disable_ssh"} {
			assert.NotContains(t, server, key)
		}
		assert.Equal(t, map[string]any{
			"REVERSE_PROXY_TRUSTED_PROXIES": "*", "MIN_PASSWORD_LENGTH": "12",
			"INSTALL_LOCK": true, "DISABLE_GIT_HOOKS": true, "IMPORT_LOCAL_PATHS": false,
			"ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET": true,
		}, config["security"])
		assert.Equal(t, map[string]any{"AVATAR_MAX_WIDTH": "1024"}, config["picture"])
		assert.Equal(t, map[string]any{"ROOT": "/data/git/repositories", "DEFAULT_PRIVATE": "true"}, config["repository"])
		assert.Equal(t, map[string]any{"remote.MyRemote.url": "https://example.com/repo.git"}, config["git.config"])
	})

	t.Run("ReverseProxyAuthentication_ExplicitlyDisabled", func(t *testing.T) {
		for _, cfg := range []vshnv1.VSHNForgejoConfig{
			{},
			{Service: map[string]string{
				"ENABLE_REVERSE_PROXY_AUTHENTICATION": "true", "enable_reverse_proxy_authentication": "true",
				" Enable_Reverse_Proxy_Authentication_API ": "true", "ENABLE_REVERSE_PROXY_AUTO_REGISTRATION": "true",
				"ENABLE_REVERSE_PROXY_EMAIL": "true", "ENABLE_REVERSE_PROXY_FULL_NAME": "true",
				"DISABLE_REGISTRATION": "true",
			}},
		} {
			config, _ := configFromRelease(t, cfg)
			expected := map[string]any{
				"ENABLE_REVERSE_PROXY_AUTHENTICATION": false, "ENABLE_REVERSE_PROXY_AUTHENTICATION_API": false,
				"ENABLE_REVERSE_PROXY_AUTO_REGISTRATION": false, "ENABLE_REVERSE_PROXY_EMAIL": false, "ENABLE_REVERSE_PROXY_FULL_NAME": false,
			}
			if cfg.Service != nil {
				expected["DISABLE_REGISTRATION"] = "true"
			}
			assert.Equal(t, expected, config["service"])
		}
	})

	t.Run("TokenSigningOverrides_Dropped", func(t *testing.T) {
		for _, keyCase := range []struct {
			name string
			key  func(string) string
		}{
			{"uppercase", strings.ToUpper},
			{"lowercase", strings.ToLower},
			{"whitespace", func(key string) string { return " " + key + " " }},
			// _0X5F_ decodes to an underscore.
			{"encoded", func(key string) string { return strings.ReplaceAll(key, "_", "_0X5F_") }},
			{"file", func(key string) string { return key + "__FILE" }},
		} {
			t.Run(keyCase.name, func(t *testing.T) {
				oauth2 := map[string]string{"ENABLED": "false"}
				server := map[string]string{"LANDING_PAGE": "explore"}
				for key, value := range map[string]string{
					"JWT_SECRET": "replacement", "JWT_SECRET_URI": "file:/tmp/secret",
					"JWT_SIGNING_ALGORITHM": "HS512", "JWT_SIGNING_PRIVATE_KEY_FILE": "/tmp/private.pem",
				} {
					oauth2[keyCase.key(key)] = value
					server[keyCase.key("LFS_"+key)] = value
				}
				config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{OAuth2: oauth2, Server: server})
				assert.Equal(t, map[string]any{"ENABLED": "false"}, config["oauth2"])
				assert.Equal(t, "explore", config["server"].(map[string]any)["LANDING_PAGE"])
				for key := range server {
					if key != "LANDING_PAGE" {
						assert.NotContains(t, config["server"], key)
					}
				}
			})
		}
	})

	t.Run("ValueNewline_Dropped", func(t *testing.T) {
		config, fqdn := configFromRelease(t, vshnv1.VSHNForgejoConfig{
			Server: map[string]string{"LANDING_PAGE": "login\nDISABLE_SSH=false"},
			UI:     map[string]string{"THEMES": "forgejo\r\nSHOW_USER_EMAIL=true", "DEFAULT_THEME": "forgejo"},
			// Sort after the managed key to exercise the chart's last-write-wins behavior.
			Security: map[string]string{"ZZZ_CARRIER": "true\nDISABLE_GIT_HOOKS=false"},
		})

		server := config["server"].(map[string]any)
		assert.Equal(t, true, server["DISABLE_SSH"])
		assert.Equal(t, fqdn, server["DOMAIN"])
		// Rejected overrides preserve defaults.
		assert.Equal(t, "login", server["LANDING_PAGE"])
		assert.Equal(t, map[string]any{"DEFAULT_THEME": "forgejo"}, config["ui"])
		assert.NotContains(t, config["security"], "ZZZ_CARRIER")
		assert.Equal(t, true, config["security"].(map[string]any)["DISABLE_GIT_HOOKS"])
	})

	t.Run("UnmanagedSections_IndirectKeysDropped", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{
			Mailer: map[string]string{
				"PROTOCOL__FILE": "/tmp/protocol", "PROTOCOL_0X5F_URI": "smtp",
				"SMTP_ADDR": "mail.example.com",
			},
			GitConfig: map[string]string{
				"core.hooksPath__FILE": "/tmp/hooks", "remote.origin.url": "https://example.com/repo.git",
			},
		})

		assert.Equal(t, map[string]any{"SMTP_ADDR": "mail.example.com"}, config["mailer"])
		assert.Equal(t, map[string]any{"remote.origin.url": "https://example.com/repo.git"}, config["git.config"])
	})

	t.Run("DefaultsWithQuotedValues_Kept", func(t *testing.T) {
		config, _ := configFromRelease(t, vshnv1.VSHNForgejoConfig{})
		assert.Equal(t, "'{\"size\":100, \"recent_ratio\":0.25, \"ghost_ratio\":0.5}'",
			config["cache"].(map[string]any)["HOST"])
	})

	t.Run("SectionLessAndLateValues_Scrubbed", func(t *testing.T) {
		svc, comp, secretName := bootstrapTest(t)
		comp.Spec.Parameters.Service.ForgejoSettings.AppName = "Forge\nDISABLE_SSH=false"
		comp.Spec.Parameters.Service.AdminEmail = "admin@example.com\nINSTALL_LOCK=false"
		assert.NoError(t, addForgejo(context.TODO(), svc, comp, secretName))

		release := &xhelmv1.Release{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()))
		config := getReleaseValues(t, *release)["gitea"].(map[string]any)["config"].(map[string]any)

		assert.NotContains(t, config, "APP_NAME")
		assert.NotContains(t, config["admin"], "ADMIN_EMAIL")
		assert.Equal(t, true, config["server"].(map[string]any)["DISABLE_SSH"])
		assert.Equal(t, true, config["security"].(map[string]any)["INSTALL_LOCK"])
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

		route := &gatewayv1.HTTPRoute{}
		assert.NoError(t, svc.GetDesiredKubeObject(route, comp.GetName()+"-httproute"))
		assert.Equal(t, gatewayv1.Duration("1h"), *route.Spec.Rules[0].Timeouts.Request)
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
