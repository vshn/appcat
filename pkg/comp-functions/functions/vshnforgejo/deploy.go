package vshnforgejo

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
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

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{"password"}, common.AddStaticFieldToSecret(map[string]string{
		"username": "gitea_admin",
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

	svc.Log.Info("Adding forgejo release")
	err = addForgejo(ctx, svc, comp, secretName)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add forgejo release: %s", err))
	}

	return nil
}

func addForgejo(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, secretName string) error {

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
		"gitea": map[string]any{
			"admin": map[string]any{
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
					"archive_cleanup": map[string]any{
						"SCHEDULE":   "@hourly",
						"OLDER_THAN": "2h",
					},
				},
				"database": map[string]any{
					"DB_TYPE": "sqlite3",
				},
				"image": map[string]any{
					"tag": comp.Spec.Parameters.Service.Version,
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
		"ingress": map[string]any{
			"annotations": map[string]string{
				"cert-manager.io/cluster-issuer": "letsencrypt-staging",
			},
			"enabled": true,
			"hosts":   []map[string]any{},
			"tls": []map[string]any{
				{
					"hosts":      []string{},
					"secretName": "forgejo-tls",
				},
			},
		},
		"persistance": map[string]any{
			"enabled": true,
		},
		"postgresql": map[string]any{
			"enabled": false,
		},
		"postgresql-ha": map[string]any{
			"enabled": false,
		},
		"redis": map[string]any{
			"enabled": false,
		},
		"redis-cluster": map[string]any{
			"enabled": false,
		},
		"strategy": map[string]any{
			"type": "Recreate",
		},
	}

	for _, host := range comp.Spec.Parameters.Service.FQDN {
		values["ingress"].(map[string]any)["hosts"] = append(values["ingress"].(map[string]any)["hosts"].([]map[string]any), map[string]any{
			"host": host,
			"paths": []map[string]any{
				{
					"path":     "/",
					"pathType": "Prefix",
				},
			},
		})
		values["ingress"].(map[string]any)["tls"].([]map[string]any)[0]["hosts"] = append(values["ingress"].(map[string]any)["tls"].([]map[string]any)[0]["hosts"].([]string), host)
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		values["containerSecurityContext"] = securityContext["containerSecurityContext"]
		values["podSecurityContext"] = securityContext["podSecurityContext"]
	}

	if comp.Spec.Parameters.Service.AdminEmail != "" {
		values["gitea"].(map[string]any)["config"].(map[string]any)["admin"].(map[string]any)["ADMIN_EMAIL"] = comp.Spec.Parameters.Service.AdminEmail
	}

	release, err := common.NewRelease(ctx, svc, comp, values)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(release)
}
