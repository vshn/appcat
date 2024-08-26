package vshnnextcloud

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dario.cat/mergo"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"
)

const (
	pgInstanceNameSuffix = "-pg"
	pgSecretName         = "pg-creds"

	adminUserSecretField          = "adminUser"
	adminPWSecretField            = "adminPassword"
	adminPWConnectionDetailsField = "NEXTCLOUD_PASSWORD"
	adminConnectionDetailsField   = "NEXTCLOUD_USERNAME"
	hostConnectionDetailsField    = "NEXTCLOUD_HOST"
	urlConnectionDetailsField     = "NEXTCLOUD_URL"
	serviceSuffix                 = "nextcloud"
)

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

	if comp.Spec.Parameters.Service.UseExternalPostgreSQL {
		svc.Log.Info("Adding postgresql instance")
		err = common.NewPostgreSQLDependencyBuilder(svc, comp).
			AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
			SetCustomMaintenanceSchedule(comp.Spec.Parameters.Maintenance.TimeOfDay.AddTime(20 * time.Minute)).
			CreateDependency()
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot create postgresql instance: %s", err))
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

		ready, err := svc.WaitForDependenciesWithConnectionDetails(comp.GetName(), resourceCDMap)
		if err != nil {
			// We're returning a fatal here, so in case something is wrong we won't delete anything by mistake.
			return runtime.NewFatalResult(err)
		} else if !ready {
			return runtime.NewWarningResult("postgresql instance not yet ready")
		}
	}

	svc.Log.Info("Adding release")

	adminSecret, err := common.AddCredentialsSecret(comp, svc, []string{adminPWSecretField}, common.AddStaticFieldToSecret(map[string]string{adminUserSecretField: "admin"}))
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

	err = addRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	return nil
}

func addPostgreSQL(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) error {
	// Unfortunately k8up and stackgres backups don't match up very well...
	// if no daily backup is set we just do the default.
	retention := 6
	if comp.Spec.Parameters.Backup.Retention.KeepDaily != 0 {
		retention = comp.Spec.Parameters.Backup.Retention.KeepDaily
	}

	params := &vshnv1.VSHNPostgreSQLParameters{
		Size:        comp.Spec.Parameters.Size,
		Maintenance: comp.GetFullMaintenanceSchedule(),
		Backup: vshnv1.VSHNPostgreSQLBackup{
			Retention:          retention,
			DeletionProtection: ptr.To(true),
			DeletionRetention:  7,
		},
		Monitoring: comp.Spec.Parameters.Monitoring,
	}

	if comp.Spec.Parameters.Service.PostgreSQLParameters != nil {
		err := mergo.Merge(params, comp.Spec.Parameters.Service.PostgreSQLParameters, mergo.WithOverride)
		if err != nil {
			return err
		}

		// Mergo doesn't override non-default values with default values. So
		// changing true to false is not possible with a merge.
		// This is a small hack to fix this.
		// `mergo.WithOverwriteWithEmptyValue` opens a new can of worms, so it's
		// not used here. https://github.com/darccio/mergo/issues/249
		if comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection != nil {
			params.Backup.DeletionProtection = comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection
		}
	}
	// We need to set this after the merge, as the default instance count for PostgreSQL is always 1
	// and would therefore override any value we set before the merge.
	params.Instances = comp.Spec.Parameters.Instances

	pg := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + pgInstanceNameSuffix,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: *params,
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      pgSecretName,
					Namespace: comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err := common.CustomCreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-nextcloud", false, svc)
	if err != nil {
		return err
	}

	err = common.DisableBilling(pg.GetInstanceNamespace(), svc)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(pg)
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret)
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

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret string) (map[string]any, error) {
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

	if comp.Spec.Parameters.Service.UseExternalPostgreSQL {
		cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + pgInstanceNameSuffix)
		if err != nil {
			return nil, err
		}
		externalDb = map[string]any{
			"enabled": true,
			"type":    "postgresql",
			"existingSecret": map[string]any{
				"enabled":     true,
				"secretName":  pgSecretName,
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
	}

	configString := svc.Config.Data["isOpenshift"]
	isOpenShift, err := strconv.ParseBool(configString)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if this is an OpenShift cluster or not: %w", err)
	}
	securityContext := map[string]any{}
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
	}

	updatedNextcloudConfig := setBackgroundJobMaintenance(comp.Spec.Parameters.Maintenance.GetMaintenanceTimeOfDay(), nextcloudConfig)
	values = map[string]any{
		"nextcloud": map[string]any{
			"host": comp.Spec.Parameters.Service.FQDN,
			"existingSecret": map[string]any{
				"enabled":     true,
				"secretName":  adminSecret,
				"usernameKey": adminUserSecretField,
				"passwordKey": adminPWSecretField,
			},
			"configs": map[string]string{
				"vshn-nextcloud.config.php": updatedNextcloudConfig,
			},
			"containerPort":      8080,
			"podSecurityContext": securityContext,
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
		"metrics": map[string]any{
			"enabled": true,
		},
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
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud, adminSecret string) (*xhelmv1.Release, error) {
	values, err := newValues(ctx, svc, comp, adminSecret)
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

	release, err := common.NewRelease(ctx, svc, comp, values)

	release.Spec.ForProvider.Chart.Name = "nextcloud"

	return release, err
}

func addApacheConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNNextcloud) error {

	cm := &v1.ConfigMap{
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

	cm := &v1.ConfigMap{
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
