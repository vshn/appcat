package vshnforgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// DeployForgejo deploys a Forgejo instance via the Helm Chart.
func DeployForgejo(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules!")
	err = common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
	}

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{
		"username": "forgejo_admin",
	}))
	if err != nil {
		return runtime.NewWarningResult("cannot add credentials secret")
	}

	connDetails, err := svc.GetObservedComposedResourceConnectionDetails(secretName)
	if err != nil {
		return runtime.NewWarningResult("cannot get connection details")
	}

	svc.SetConnectionDetail("FORGEJO_USERNAME", connDetails["username"])
	svc.SetConnectionDetail("FORGEJO_PASSWORD", connDetails["password"])
	// "," as separator for multiple FQDNs, it's better than anything else as highlighting with mouse in terminal works well
	// result is fqdn[0],fqdn[1],fqdn[2]
	svc.SetConnectionDetail("FORGEJO_URL", []byte(strings.Join(comp.Spec.Parameters.Service.FQDN, ",")))
	svc.SetConnectionDetail("FORGEJO_HOST", []byte(fmt.Sprintf("%s-http.%s.svc", helmFullname(comp), comp.GetInstanceNamespace())))

	svc.Log.Info("Adding forgejo release")
	err = addForgejo(ctx, svc, comp, secretName)
	if errors.Is(err, common.ErrHTTPGatewayNotConfigured) {
		return runtime.NewFatalResult(fmt.Errorf("cannot add forgejo release: %w", err))
	}
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add forgejo release: %s", err))
	}

	svc.Log.Info("Have added forgejo release!")

	return nil
}

func addForgejo(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, secretName string) error {

	if len(comp.Spec.Parameters.Service.FQDN) == 0 {
		return fmt.Errorf("must supply at least one FQDN")
	}

	securityContext := map[string]any{}
	if svc.Config.Data["isOpenshift"] == "true" {
		securityContext = map[string]any{
			"runAsUser":                nil,
			"allowPrivilegeEscalation": false,
			"podSecurityContext": map[string]any{
				"fsGroup": nil,
			},
			"containerSecurityContext": map[string]any{
				"runAsUser":                nil,
				"allowPrivilegeEscalation": false,
			},
			"capabilities": map[string]any{
				"drop": []string{
					"ALL",
				},
			},
		}
	}

	values := map[string]any{
		"replicaCount": comp.GetInstances(),
		"gitea": map[string]any{
			"admin": map[string]any{
				"email":          "forgejo@local.domain",
				"username":       "forgejo_admin",
				"existingSecret": secretName,
			},
			"config": map[string]any{
				"admin": map[string]any{
					"SEND_NOTIFICATION_EMAIL_ON_NEW_USER": true,
				},
				"cache": map[string]any{
					"ADAPTER": "twoqueue",
					"HOST":    "'{\"size\":100, \"recent_ratio\":0.25, \"ghost_ratio\":0.5}'",
				},
				"cron": map[string]any{
					"ENABLED": true,
				},
				"cron.archive_cleanup": map[string]any{
					"SCHEDULE":   "@hourly",
					"OLDER_THAN": "2h",
				},
				"database": map[string]any{
					"DB_TYPE":             "sqlite3",
					"SQLITE_JOURNAL_MODE": "WAL",
				},
				"indexer": map[string]any{
					"ISSUE_INDEXER_TYPE":   "bleve",
					"REPO_INDEXER_ENABLED": true,
				},
				"lfs": map[string]any{
					"PATH": "/data/git/lfs",
				},
				"log": map[string]any{
					"LEVEL": "info",
				},
				"packages": map[string]any{
					"LIMIT_SIZE_CONTAINER": "2 GiB",
				},
				"queue": map[string]any{
					"TYPE": "level",
				},
				"repository": map[string]any{
					"ROOT": "/data/git/repositories",
				},
				"security": map[string]any{
					"REVERSE_PROXY_TRUSTED_PROXIES": "*",
				},
				"server": map[string]any{
					"DOMAIN":           comp.Spec.Parameters.Service.FQDN[0],
					"ROOT_URL":         "https://" + comp.Spec.Parameters.Service.FQDN[0],
					"DISABLE_SSH":      true,
					"LANDING_PAGE":     "login",
					"LFS_START_SERVER": true,
					"MINIMUM_KEY_SIZE": true,
					"OFFLINE_MODE":     true,
				},
				"session": map[string]any{
					"PROVIDER": "memory",
				},
			},
			"metrics": map[string]any{
				"enabled": true,
			},
			"serviceMonitor": map[string]any{
				"enabled": true,
			},
		},
		"image": map[string]any{
			"tag": comp.Spec.Parameters.Service.MajorVersion,
		},
		"persistence": map[string]any{
			"enabled": true,
		},
		"extraVolumes": []map[string]any{{
			"name":     "backup-scratch",
			"emptyDir": map[string]any{},
		}},
		"extraContainerVolumeMounts": []map[string]string{{
			"name":      "backup-scratch",
			"mountPath": "/tmp/backup",
		}},
		"postgresql": map[string]any{
			"enabled": false,
		},
		"postgresql-ha": map[string]any{
			"enabled": false,
		},
		"valkey": map[string]any{
			"enabled": false,
		},
		"valkey-cluster": map[string]any{
			"enabled": false,
		},
		"strategy": map[string]any{
			"type": "Recreate",
		},
		// This will be overwritten by setResources() later
		"resources": map[string]any{
			"limits":   map[string]any{},
			"requests": map[string]any{},
		},
	}

	if svc.Config.Data["imageRegistry"] == "" {
		err := common.SetNestedObjectValue(values, []string{"image", "registry"}, svc.Config.Data["imageRegistry"])
		if err != nil {
			return err
		}
	}

	// Compute resources
	svc.Log.Info("Fetching and setting compute resources")
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return fmt.Errorf("could not fetch plans from config: %w", err)
	}
	svc.Log.Info("Got resources from plan", "plan", plan, "resources", resources)

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		svc.Log.Error(fmt.Errorf("could not get resources"), "errors", errors.Join(errs...))
	}
	svc.Log.Info("Final resources to use", "resources", res)

	err = setResources(values, res)
	if err != nil {
		return fmt.Errorf("cannot set resources: %w", err)
	}

	// App name
	appName := comp.Spec.Parameters.Service.ForgejoSettings.AppName
	if appName != "" {
		common.SetNestedObjectValue(values, []string{"gitea", "config", "APP_NAME"}, appName)
	}

	svc.Log.Info("Updating forgejo settings")
	var objmap map[string]any
	o, err := json.Marshal(comp.Spec.Parameters.Service.ForgejoSettings.Config)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(o, &objmap); err != nil {
		return err
	}

	// Filter before merging so rejected entries leave defaults intact.
	dropUnsafeForgejoConfig(objmap)

	giteaConfig := values["gitea"].(map[string]any)["config"].(map[string]any)
	for section, v := range objmap {
		if v == nil {
			continue
		}

		if userSection, ok := v.(map[string]any); ok {
			if baseSection, ok := giteaConfig[section].(map[string]any); ok {
				maps.Copy(baseSection, userSection)
				continue
			}
		}
		giteaConfig[section] = v
	}

	// Ingress / HTTPRoute
	svcNameSuffix := "http"
	if !strings.Contains(comp.GetName(), "forgejo") {
		svcNameSuffix = "forgejo-" + svcNameSuffix
	}

	if common.IsHTTPRouteMode(svc) {
		svc.Log.Info("Adding HTTPRoute for Forgejo")

		// Large repo clone/push/LFS outlives the gateway's 15s default request timeout.
		requestTimeout := gatewayv1.Duration("1h")

		if err := common.ApplyHTTPRoute(comp, svc, common.HTTPRouteConfig{
			FQDNs: comp.Spec.Parameters.Service.FQDN,
			ServiceConfig: common.IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 3000,
			},
			Timeouts: &gatewayv1.HTTPRouteTimeouts{
				Request: &requestTimeout,
			},
		}); err != nil {
			return err
		}
	} else {
		svc.Log.Info("Adding ingress")
		ingressConfig := common.IngressConfig{
			FQDNs: comp.Spec.Parameters.Service.FQDN,
			ServiceConfig: common.IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 3000,
			},
			TlsCertBaseName: "forgejo",
		}
		ingresses, err := common.GenerateBundledIngresses(comp, svc, ingressConfig)
		if err != nil {
			return err
		}
		err = common.CreateIngresses(comp, svc, ingresses)
		if err != nil {
			return err
		}
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		values["containerSecurityContext"] = securityContext["containerSecurityContext"]
		values["podSecurityContext"] = securityContext["podSecurityContext"]
	}

	if comp.Spec.Parameters.Service.AdminEmail != "" {
		err := common.SetNestedObjectValue(values, []string{"gitea", "config", "admin", "ADMIN_EMAIL"}, comp.Spec.Parameters.Service.AdminEmail)
		if err != nil {
			return err
		}
	}

	// Run after all tenant values, including APP_NAME and ADMIN_EMAIL, are assigned.
	protectForgejoConfig(giteaConfig, comp.Spec.Parameters.Service.FQDN[0])

	// NewRelease doesn't actually use resName, but rather comp.GetName() as is
	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName())
	if err != nil {
		return fmt.Errorf("cannot get observed release values: %w", err)
	}
	releaseTag, err := maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.MajorVersion, values, observedValues, []string{"image", "tag"}, comp.Spec.Parameters.Maintenance.PinImageTag)
	if err != nil {
		return fmt.Errorf("cannot set forgejo version for release: %w", err)
	}

	// Update status with current release tag
	if releaseTag != "" {
		comp.Status.CurrentReleaseTag = releaseTag
		maintenance.UpdatePinImageTagStatus(comp.Spec.Parameters.Maintenance.PinImageTag, &comp.Status.PinImageTagStatus)
		if err := svc.SetDesiredCompositeStatus(comp); err != nil {
			svc.Log.Error(err, "cannot update CurrentReleaseTag in status")
		}
	}

	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-release")
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(release)
}

// The chart merges into an existing app.ini, so safety switches must be explicit.
func protectForgejoConfig(config map[string]any, fqdn string) {
	dropUnsafeForgejoConfig(config)

	for section, managed := range map[string]map[string]any{
		"server": {
			"DOMAIN":      fqdn,
			"ROOT_URL":    "https://" + fqdn,
			"DISABLE_SSH": true,
		},
		"security": {
			"REVERSE_PROXY_TRUSTED_PROXIES":            "*",
			"INSTALL_LOCK":                             true,
			"DISABLE_GIT_HOOKS":                        true,
			"IMPORT_LOCAL_PATHS":                       false,
			"ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET": true,
		},
		"service": {
			// All proxies are trusted, so header authentication must stay disabled.
			"ENABLE_REVERSE_PROXY_AUTHENTICATION":     false,
			"ENABLE_REVERSE_PROXY_AUTHENTICATION_API": false,
			"ENABLE_REVERSE_PROXY_AUTO_REGISTRATION":  false,
			"ENABLE_REVERSE_PROXY_EMAIL":              false,
			"ENABLE_REVERSE_PROXY_FULL_NAME":          false,
		},
		"repository": {"ROOT": "/data/git/repositories"},
	} {
		sectionConfig, ok := config[section].(map[string]any)
		if !ok {
			sectionConfig = map[string]any{}
			config[section] = sectionConfig
		}
		dropForgejoConfigKeys(sectionConfig, slices.Collect(maps.Keys(managed)))
		maps.Copy(sectionConfig, managed)
	}

	// Preserve chart-generated secrets and reject alternate secret sources.
	for section, keys := range map[string][]string{
		"server": {"LFS_JWT_SECRET", "LFS_JWT_SECRET_URI", "LFS_JWT_SIGNING_ALGORITHM", "LFS_JWT_SIGNING_PRIVATE_KEY_FILE"},
		"security": {
			"REVERSE_PROXY_AUTHENTICATION_USER", "REVERSE_PROXY_AUTHENTICATION_EMAIL",
			"REVERSE_PROXY_AUTHENTICATION_FULL_NAME", "REVERSE_PROXY_LIMIT",
			"SECRET_KEY", "SECRET_KEY_URI", "INTERNAL_TOKEN", "INTERNAL_TOKEN_URI",
		},
		"oauth2": {"JWT_SECRET", "JWT_SECRET_URI", "JWT_SIGNING_ALGORITHM", "JWT_SIGNING_PRIVATE_KEY_FILE"},
		"picture": {
			// Leave avatar storage to the defaults.
			"AVATAR_UPLOAD_PATH", "AVATAR_STORAGE_TYPE",
			"REPOSITORY_AVATAR_UPLOAD_PATH", "REPOSITORY_AVATAR_STORAGE_TYPE",
		},
	} {
		if sectionConfig, ok := config[section].(map[string]any); ok {
			dropForgejoConfigKeys(sectionConfig, keys)
		}
	}
}

func dropForgejoConfigKeys(config map[string]any, protected []string) {
	maps.DeleteFunc(config, func(key string, _ any) bool {
		// Match the chart's key normalization.
		return slices.Contains(protected, strings.ToUpper(strings.TrimSpace(key)))
	})
}

func dropUnsafeForgejoConfig(config map[string]any) {
	for key, value := range config {
		if section, ok := value.(map[string]any); ok {
			maps.DeleteFunc(section, isUnsafeForgejoEntry)
			continue
		}
		if isUnsafeForgejoEntry(key, value) {
			delete(config, key)
		}
	}
}

func isUnsafeForgejoEntry(key string, value any) bool {
	// Reject encoded names, file reads, and injection into the chart's KEY=VALUE lines.
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if strings.Contains(normalized, "_0X") || strings.HasSuffix(normalized, "__FILE") ||
		strings.ContainsAny(key, "\"'\\\r\n=") {
		return true
	}
	text, ok := value.(string)
	return ok && strings.ContainsAny(text, "\r\n")
}

// Set compute resources in the values map
func setResources(values map[string]any, resources common.Resources) error {
	err := common.SetNestedObjectValue(values, []string{"resources", "limits", "cpu"}, resources.CPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"resources", "requests", "cpu"}, resources.ReqCPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"resources", "limits", "memory"}, resources.Mem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"resources", "requests", "memory"}, resources.ReqMem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"persistence", "size"}, resources.Disk.String())
	if err != nil {
		return fmt.Errorf("cannot set disk size: %w", err)
	}

	return nil
}

// helmFullname mirrors the Helm fullname template used by the Forgejo chart.
// If the release name already contains the chart name ("forgejo"), the fullname
// is just the release name; otherwise it appends "-forgejo".
func helmFullname(comp *vshnv1.VSHNForgejo) string {
	name := comp.GetName()
	chart := comp.GetServiceName()
	if strings.Contains(name, chart) {
		return name
	}
	return name + "-" + chart
}
