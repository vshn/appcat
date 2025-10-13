package vshnnextcloud

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dario.cat/mergo"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pgInstanceNameSuffix = "-pg"

	adminUserSecretField          = "adminUser"
	adminPWSecretField            = "adminPassword"
	adminPWConnectionDetailsField = "NEXTCLOUD_PASSWORD"
	adminConnectionDetailsField   = "NEXTCLOUD_USERNAME"
	hostConnectionDetailsField    = "NEXTCLOUD_HOST"
	urlConnectionDetailsField     = "NEXTCLOUD_URL"
	collaboraHostField            = "COLLABORA_HOST"
	serviceSuffix                 = "nextcloud"
)

//go:embed files/install-collabora.sh
var installCollabora string

//go:embed files/000-default.conf
var apacheVhostConfig string

//go:embed files/vshn-nextcloud.config.php
var nextcloudConfig string

//go:embed files/nextcloud-post-installation.sh
var nextcloudPostInstallation string

//go:embed files/nextcloud-post-upgrade.sh
var nextcloudPostUpgrade string

// DeployNextcloud deploys a nexctloud instance via the codecentric Helm Chart.
func DeployNextcloud(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
	}

	pgSecret, err := configureDatabase(ctx, comp, svc)
	if err != nil {
		runtime.NewWarningResult(fmt.Sprintf("cannot configure database: %s", err))
	}

	svc.Log.Info("Adding release")

	adminSecret, err := common.AddCredentialsSecret(comp, svc, []string{adminPWSecretField}, common.DisallowDeletion, common.AddStaticFieldToSecret(map[string]string{adminUserSecretField: "admin"}))
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot generate admin secret: %s", err))
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed connection details for nextcloud admin: %s", err))
	}

	hostname := comp.GetName()
	if !strings.Contains(hostname, serviceSuffix) {
		hostname = hostname + "-" + serviceSuffix
	}

	if comp.Spec.Parameters.Service.Collabora.Enabled {
		svc.SetConnectionDetail(collaboraHostField, []byte(fmt.Sprintf("%s-collabora-code.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace())))
	}

	svc.SetConnectionDetail(adminPWConnectionDetailsField, cd[adminPWSecretField])
	svc.SetConnectionDetail(adminConnectionDetailsField, cd[adminUserSecretField])
	svc.SetConnectionDetail(hostConnectionDetailsField, []byte(fmt.Sprintf("%s.%s.svc.cluster.local", hostname, comp.GetInstanceNamespace())))

	err = addApacheConfig(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add configmap for apache: %s", err))
	}

	err = addNextcloudHooks(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("Cannot add configmap for Nextcloud: %s", err))
	}

	err = createClusterRoleBinding(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create cluster role binding: %s", err))
	}

	err = createInstallCollaboraConfigMap(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("Cannot add configmap for Collabora: %s", err))
	}

	err = addRelease(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	return nil
}

func configureDatabase(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) (pgSecret string, err error) {
	if comp.Spec.Parameters.Service.UseExternalPostgreSQL {
		if comp.Spec.Parameters.Service.ExistingPGConnectionSecret != "" {
			return establishConnectionToExistingPG(ctx, comp, svc)
		} else {
			return createNewPGService(comp, svc)
		}
	}
	return "", nil
}

func createNewPGService(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) (pgSecret string, err error) {
	var pgTime vshnv1.TimeOfDay
	pgTime.SetTime(comp.GetMaintenanceTimeOfDay().GetTime().Add(20 * time.Minute))

	pgBouncerConfig, pgSettings, pgDiskSize, err := getObservedPostgresSettings(svc, comp)
	if err != nil {
		return "", fmt.Errorf("cannot get observed postgres settings: %s", err)
	}

	svc.Log.Info("Adding postgresql instance")

	pgBuilder := common.NewPostgreSQLDependencyBuilder(svc, comp).
		AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
		AddPGBouncerConfig(pgBouncerConfig).
		AddPGSettings(pgSettings).
		SetCustomMaintenanceSchedule(pgTime)

	if pgDiskSize != "" {
		pgBuilder.SetDiskSize(pgDiskSize)
	}

	pgSecret, err = pgBuilder.CreateDependency()
	if err != nil {
		return "", fmt.Errorf("cannot create postgresql instance: %s", err)
	}

	svc.Log.Info("Checking readiness of cluster")

	resourceCDMap := map[string][]string{
		comp.GetName() + pgInstanceNameSuffix: {
			vshnpostgres.PostgresqlHost,
			vshnpostgres.PostgresqlPort,
			vshnpostgres.PostgresqlDb,
			vshnpostgres.PostgresqlUser,
			vshnpostgres.PostgresqlPassword,
		},
	}

	ready, err := svc.WaitForObservedDependenciesWithConnectionDetails(comp.GetName(), resourceCDMap)
	if err != nil {
		return "", fmt.Errorf("cannot wait for observed dependendies: %s", err)
	} else if !ready {
		return "", fmt.Errorf("postgresql instance not yet ready")
	}
	return pgSecret, nil
}

func establishConnectionToExistingPG(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) (pgSecret string, err error) {
	svc.Log.Info("Connecting to existing postgresql instance")

	existingCD := comp.Spec.Parameters.Service.ExistingPGConnectionSecret
	cNamespace := comp.GetClaimNamespace()
	iNamespace := comp.GetInstanceNamespace()
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      existingCD,
			Namespace: cNamespace,
		},
	}
	s, err := svc.CopyKubeResource(ctx, existingSecret, comp.GetName()+"-postgresql-connection-secret", existingCD, cNamespace, iNamespace)
	if err != nil || len(s.(*corev1.Secret).Data) == 0 {
		return "", fmt.Errorf("existing postgres connection secret not ready: %s", err)
	}
	pgInstanceNamespace, err := getPgInstanceNamespace(string(s.(*corev1.Secret).Data[vshnpostgres.PostgresqlHost]))
	if err != nil {
		return "", fmt.Errorf("cannot get pgInstanceNamespace: %s", err)
	}
	err = common.CustomCreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pgInstanceNamespace, "allow-from-"+comp.GetInstanceNamespace(), comp.GetName()+"-netpol-allow-to-pg", false, svc)
	if err != nil {
		return "", fmt.Errorf("cannot configure network policy resource in Postgres service: %s", err)
	}
	pgSecret = s.GetName()
	return pgSecret, nil
}

func getPgInstanceNamespace(host string) (string, error) {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid host format: %s", host)
	}
	return parts[1], nil
}

func getObservedPostgresSettings(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) (map[string]string, map[string]string, string, error) {
	pgBouncerConfig := map[string]string{
		"max_client_conn": "1000",
	}
	pgSettings := map[string]string{
		"max_connections": "1000",
	}
	pgDiskSize := ""

	if !comp.Spec.Parameters.Service.UseExternalPostgreSQL {
		return pgBouncerConfig, pgSettings, pgDiskSize, nil
	}

	existing := &vshnv1.XVSHNPostgreSQL{}
	err := svc.GetObservedComposedResource(existing, comp.GetName()+pgInstanceNameSuffix)

	switch {
	case err == nil:
		if existing.Spec.Parameters.Size.Disk != "" {
			pgDiskSize = existing.Spec.Parameters.Size.Disk
		}

		if existing.Spec.Parameters.Service.PgBouncerSettings != nil {
			if bouncer := existing.Spec.Parameters.Service.PgBouncerSettings.Pgbouncer.Raw; len(bouncer) > 0 {
				var rawBouncer map[string]interface{}
				if err := json.Unmarshal(bouncer, &rawBouncer); err != nil {
					return nil, nil, "", fmt.Errorf("cannot unmarshal existing pgbouncersettings: %w", err)
				}

				bouncerSettings := make(map[string]string)
				for k, v := range rawBouncer {
					bouncerSettings[k] = fmt.Sprint(v)
				}

				if err := mergo.Merge(&pgBouncerConfig, bouncerSettings, mergo.WithOverride); err != nil {
					return nil, nil, "", fmt.Errorf("cannot merge existing pgbouncerconfig: %w", err)
				}
			}
		}

		if settings := existing.Spec.Parameters.Service.PostgreSQLSettings.Raw; len(settings) > 0 {
			var rawSettings map[string]interface{}
			if err := json.Unmarshal(settings, &rawSettings); err != nil {
				return nil, nil, "", fmt.Errorf("cannot unmarshal existing pgsettings: %w", err)
			}

			pgSettingsMap := make(map[string]string)
			for k, v := range rawSettings {
				pgSettingsMap[k] = fmt.Sprint(v)
			}

			if err := mergo.Merge(&pgSettings, pgSettingsMap, mergo.WithOverride); err != nil {
				return nil, nil, "", fmt.Errorf("cannot merge existing pgsettings: %w", err)
			}
		}

	case err == runtime.ErrNotFound:
		pgDiskSize = "10Gi"

	default:
		return nil, nil, "", fmt.Errorf("cannot get observed postgres instance: %w", err)
	}

	if params := comp.Spec.Parameters.Service.PostgreSQLParameters; params != nil {
		if params.Size.Disk != "" {
			desiredDiskSize, err := resource.ParseQuantity(params.Size.Disk)
			if err != nil {
				return nil, nil, "", fmt.Errorf("cannot parse desired disk size %q: %w", params.Size.Disk, err)
			}

			currentDiskSize, err := resource.ParseQuantity(pgDiskSize)
			if err != nil {
				return nil, nil, "", fmt.Errorf("cannot parse current disk size %q: %w", pgDiskSize, err)
			}

			if desiredDiskSize.Cmp(currentDiskSize) >= 0 {
				pgDiskSize = params.Size.Disk
			}
		}

		if params.Service.PgBouncerSettings != nil && len(params.Service.PgBouncerSettings.Pgbouncer.Raw) > 0 {
			var rawBouncer map[string]interface{}
			if err := json.Unmarshal(params.Service.PgBouncerSettings.Pgbouncer.Raw, &rawBouncer); err != nil {
				return nil, nil, "", fmt.Errorf("cannot unmarshal user pgbouncersettings: %w", err)
			}

			bouncerSettings := make(map[string]string)
			for k, v := range rawBouncer {
				bouncerSettings[k] = fmt.Sprint(v)
			}

			if err := mergo.Merge(&pgBouncerConfig, bouncerSettings, mergo.WithOverride); err != nil {
				return nil, nil, "", fmt.Errorf("cannot merge user pgbouncerconfig: %w", err)
			}
		}

		if len(params.Service.PostgreSQLSettings.Raw) > 0 {
			var rawSettings map[string]interface{}
			if err := json.Unmarshal(params.Service.PostgreSQLSettings.Raw, &rawSettings); err != nil {
				return nil, nil, "", fmt.Errorf("cannot unmarshal user pgsettings: %w", err)
			}

			pgSettingsMap := make(map[string]string)
			for k, v := range rawSettings {
				pgSettingsMap[k] = fmt.Sprint(v)
			}

			if err := mergo.Merge(&pgSettings, pgSettingsMap, mergo.WithOverride); err != nil {
				return nil, nil, "", fmt.Errorf("cannot merge user pgsettings: %w", err)
			}
		}
	}

	return pgBouncerConfig, pgSettings, pgDiskSize, nil
}

func createInstallCollaboraConfigMap(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "collabora-install-script",
			Namespace: comp.GetInstanceNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
		},
		Data: map[string]string{
			"install-collabora.sh": installCollabora,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-collabora-install-script")
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret, pgSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName()+"-release")
}

func getResources(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) (common.Resources, error) {
	l := svc.Log
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return common.Resources{}, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) != 0 {
		l.Error(errors.Join(errs...), "Cannot get Resources from plan and claim")
	}
	return res, nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret, pgSecret string) (map[string]any, error) {
	values := map[string]any{}

	res, err := getResources(ctx, svc, comp)
	if err != nil {
		return nil, err
	}

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)
	if err != nil {
		return values, fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	externalDb := map[string]any{}
	extraInitContainers := []map[string]any{}

	if comp.Spec.Parameters.Service.UseExternalPostgreSQL {
		var cd map[string][]byte
		if comp.Spec.Parameters.Service.ExistingPGConnectionSecret != "" {
			secret := &corev1.Secret{}
			err := svc.GetObservedKubeObject(secret, comp.GetName()+"-postgresql-connection-secret-claim-observer")
			if err != nil {
				return nil, err
			}
			cd = secret.Data
		} else {
			cd, err = svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + pgInstanceNameSuffix)
			if err != nil {
				return nil, err
			}
		}
		externalDb = map[string]any{
			"enabled": true,
			"type":    "postgresql",
			"existingSecret": map[string]any{
				"enabled":     true,
				"secretName":  pgSecret,
				"usernameKey": "POSTGRESQL_USER",
				"passwordKey": "POSTGRESQL_PASSWORD",
				"hostKey":     "POSTGRESQL_HOST",
				"databaseKey": "POSTGRESQL_DB",
			},
			"host":     string(cd[vshnpostgres.PostgresqlHost]) + ":" + string(cd[vshnpostgres.PostgresqlPort]),
			"database": string(cd[vshnpostgres.PostgresqlDb]),
			"user":     string(cd[vshnpostgres.PostgresqlUser]),
			"password": string(cd[vshnpostgres.PostgresqlPassword]),
		}

		extraInitContainers = []map[string]any{
			{
				"name":  "dbchecker",
				"image": svc.Config.Data["busybox_image"],
				"command": []string{
					"sh",
					"-c",
					`echo 'Waiting for Database to become ready...'

              until printf "." && nc -z -w 2 ` + string(cd[vshnpostgres.PostgresqlHost]) + " " + string(cd[vshnpostgres.PostgresqlPort]) + `; do
                  sleep 2;
              done;

              echo 'Database OK ✓'`,
				},
			},
		}
	}

	configString := svc.Config.Data["isOpenshift"]
	isOpenShift, err := strconv.ParseBool(configString)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if this is an OpenShift cluster or not: %w", err)
	}
	securityContext := map[string]any{}
	podSecurityContext := map[string]any{}

	metrics := map[string]any{
		"enabled": true,
	}

	if isOpenShift {
		securityContext = map[string]any{
			"runAsUser":                nil,
			"allowPrivilegeEscalation": false,
			"capabilities": map[string]any{
				"drop": []string{
					"ALL",
				},
			},
		}
		podSecurityContext = map[string]any{
			"fsGroupChangePolicy": "OnRootMismatch",
			"seLinuxOptions": map[string]any{
				"type": "spc_t",
			},
		}

		metrics["securityContext"] = map[string]any{
			"runAsUser": nil,
		}
	}

	trustedDomain := []string{
		comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
	}
	trustedDomain = append(trustedDomain, comp.Spec.Parameters.Service.FQDN...)

	updatedNextcloudConfig := setBackgroundJobMaintenance(*comp.GetMaintenanceTimeOfDay(), nextcloudConfig)
	values = map[string]any{
		"nextcloud": map[string]any{
			"host":           comp.Spec.Parameters.Service.FQDN[0],
			"trustedDomains": trustedDomain,
			"existingSecret": map[string]any{
				"enabled":     true,
				"secretName":  adminSecret,
				"usernameKey": adminUserSecretField,
				"passwordKey": adminPWSecretField,
			},
			"configs": map[string]string{
				"vshn-nextcloud.config.php": updatedNextcloudConfig,
			},
			"extraEnv": []map[string]any{
				{
					"name":  "SKIP_MAINTENANCE",
					"value": strconv.FormatBool(comp.Spec.Parameters.Backup.SkipMaintenance),
				},
				{
					"name":  "PGSSLROOTCERT",
					"value": "/opt/pg-certs/ca.crt",
				},
				{
					"name":  "PGSSLCERT",
					"value": "/opt/pg-certs/tls.crt",
				},
				{
					"name":  "PGSSLKEY",
					"value": "/opt/pg-certs/tls.key",
				},
				{
					"name":  "PGSSLMODE",
					"value": "require",
				},
			},
			"extraInitContainers": extraInitContainers,
			"containerPort":       8080,
			"podSecurityContext":  podSecurityContext,
			"extraVolumes": []map[string]any{
				{
					"name": "apache-config",
					"configMap": map[string]string{
						"name": "apache-config",
					},
				},
				{
					"name": "nextcloud-hooks",
					"configMap": map[string]any{
						"name":        "nextcloud-hooks",
						"defaultMode": 0754,
					},
				},
				{
					"name": "collabora-install-script",
					"configMap": map[string]any{
						"name":        "collabora-install-script",
						"defaultMode": 0755,
					},
				},
				{
					"name": "pg-cert",
					"secret": map[string]any{
						"secretName":  pgSecret,
						"defaultMode": 0640,
					},
				},
			},
			"extraVolumeMounts": []map[string]any{
				{
					"name":      "apache-config",
					"mountPath": "/etc/apache2/ports.conf",
					"subPath":   "ports.conf",
				},
				{
					"name":      "apache-config",
					"mountPath": "/etc/apache2/sites-available/000-default.conf",
					"subPath":   "000-default.conf",
				},
				{
					"name":      "nextcloud-hooks",
					"mountPath": "/docker-entrypoint-hooks.d/post-installation/vshn-post-installation.sh",
					"subPath":   "vshn-post-installation.sh",
				},
				{
					"name":      "nextcloud-hooks",
					"mountPath": "/docker-entrypoint-hooks.d/post-upgrade/vshn-post-upgrade.sh",
					"subPath":   "vshn-post-upgrade.sh",
				},
				{
					"name":      "collabora-install-script",
					"mountPath": "/install-collabora.sh",
					"subPath":   "install-collabora.sh",
				},
				{
					"name":      "pg-cert",
					"mountPath": "/opt/pg-certs",
					"readOnly":  true,
				},
			},
		},
		"securityContext": securityContext,
		"internalDatabase": map[string]any{
			"enabled": !comp.Spec.Parameters.Service.UseExternalPostgreSQL,
		},
		"startupProbe": map[string]any{
			"initialDelaySeconds": "5",
			"failureThreshold":    "60",
			"enabled":             true,
		},
		"externalDatabase": externalDb,
		"metrics":          metrics,
		"resources": map[string]any{
			"requests": map[string]any{
				"memory": res.ReqMem,
				"cpu":    res.ReqCPU,
			},
			"limits": map[string]any{
				"memory": res.Mem,
				"cpu":    res.CPU,
			},
		},
		"nodeSelector": nodeSelector,

		"http": map[string]any{
			"relativePath": comp.Spec.Parameters.Service.RelativePath,
		},
		"persistence": map[string]any{
			"enabled": true,
			"size":    res.Disk,
		},
		"rbac": map[string]any{
			"enabled": true,
			"serviceaccount": map[string]any{
				"create": true,
				"name":   comp.GetName(),
			},
		},
		"cronjob": map[string]any{
			"enabled": true,
			"type":    "cronjob",
			"affinity": map[string]any{
				"podAffinity": map[string]any{
					"requiredDuringSchedulingIgnoredDuringExecution": []map[string]any{
						{
							"labelSelector": map[string]any{
								"matchExpressions": []map[string]any{
									{
										"key":      "app.kubernetes.io/name",
										"operator": "In",
										"values": []string{
											"nextcloud",
										},
									},
									{
										"key":      "app.kubernetes.io/component",
										"operator": "In",
										"values": []string{
											"app",
										},
									},
								},
							},
							"topologyKey": "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}

	if image := svc.Config.Data["nextcloud_image"]; image != "" {
		values["image"] = map[string]interface{}{
			"repository": image,
		}
	}

	if registry := svc.Config.Data["imageRegistry"]; registry != "" {
		image := fmt.Sprintf("%s/%s", registry, "xperimental/nextcloud-exporter")

		err := common.SetNestedObjectValue(values, []string{"metrics", "image"}, map[string]any{
			"repository": image,
		})
		if err != nil {
			return nil, err
		}

	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret, pgSecret string) (*xhelmv1.Release, error) {
	values, err := newValues(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return nil, err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return nil, fmt.Errorf("cannot get observed release values: %w", err)
	}

	_, err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return nil, fmt.Errorf("cannot set keycloak version for release: %w", err)
	}

	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-release")

	release.Spec.ForProvider.Chart.Name = "nextcloud"

	return release, err
}

func addApacheConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) error {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "apache-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"000-default.conf": apacheVhostConfig,
			"ports.conf":       "Listen 8080",
		},
	}

	err := svc.SetDesiredKubeObject(cm, comp.GetName()+"-apache-config")
	if err != nil {
		return err
	}
	return nil
}

func addNextcloudHooks(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) error {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nextcloud-hooks",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"vshn-post-installation.sh": nextcloudPostInstallation,
			"vshn-post-upgrade.sh":      nextcloudPostUpgrade,
		},
	}

	err := svc.SetDesiredKubeObject(cm, comp.GetName()+"-nextcloud-hooks")
	if err != nil {
		return err
	}
	return nil
}

func setBackgroundJobMaintenance(t vshnv1.TimeOfDay, nextcloudConfig string) string {
	// Start Background Job Maintenance no earlier than 20 min after the regular Maintenance
	// and no later than 1 hour and 39 min after the regular Maintenance
	backgroundJobHour := t.GetTime().Add(40 * time.Minute).Add(time.Hour).Hour()

	return strings.Replace(nextcloudConfig, "%maintenance_value%", strconv.Itoa(backgroundJobHour), 1)
}

func createClusterRoleBinding(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	if !svc.GetBoolFromCompositionConfig("isOpenshift") {
		return nil
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      comp.GetName(),
				Namespace: comp.GetInstanceNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "appcat-scc",
		},
	}
	return svc.SetDesiredKubeObject(crb, comp.GetName()+"-cluster-role-binding")
}
