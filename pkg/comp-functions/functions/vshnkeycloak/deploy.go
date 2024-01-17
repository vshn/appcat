package vshnkeycloak

import (
	"context"
	"encoding/json"
	"fmt"

	"dario.cat/mergo"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	pgInstanceNameSuffix          = "-pg"
	pgSecretName                  = "pg-creds"
	adminPWSecretField            = "password"
	adminPWConnectionDetailsField = "KEYCLOAK_PASSWORD"
	adminConnectionDetailsField   = "KEYCLOAK_USERNAME"
	hostConnectionDetailsField    = "KEYCLOAK_HOST"
	urlConnectionDetailsField     = "KEYCLOAK_URL"
	serviceSuffix                 = "keycloakx-http"
)

// DeployKeycloak deploys a keycloak instance via the codecentric Helm Chart.
func DeployKeycloak(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNKeycloak{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Adding postgresql instance")
	err = addPostgreSQL(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create postgresql instance: %s", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
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

	svc.Log.Info("Adding release")

	adminSecret, err := common.AddCredentialsSecret(comp, svc, []string{adminPWSecretField})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot generate admin secret: %s", err))
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed connection details for keycloak admin: %s", err))
	}

	svc.Log.Info("Adding Network policy for keycloak")

	sourceNS := []string{
		comp.GetClaimNamespace(),
	}
	err = common.CreateNetworkPolicy(sourceNS, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create net pol: %w", err))
	}

	svc.SetConnectionDetail(adminPWConnectionDetailsField, cd[adminPWSecretField])
	svc.SetConnectionDetail(adminConnectionDetailsField, []byte("admin"))
	svc.SetConnectionDetail(hostConnectionDetailsField, []byte(fmt.Sprintf("%s-%s.%s.svc.cluster.local", comp.GetName(), serviceSuffix, comp.GetInstanceNamespace())))

	err = addRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	return nil
}

func addPostgreSQL(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) error {
	// Unfortunately k8up and stackgres backups don't match up very well...
	// if no daily backup is set we just do the default.
	retention := 6
	if comp.Spec.Parameters.Backup.Retention.KeepDaily != 0 {
		retention = comp.Spec.Parameters.Backup.Retention.KeepDaily
	}

	configs := map[string]string{
		"ignore_startup_parameters": "extra_float_digits, search_path",
	}

	configBytes, err := json.Marshal(configs)
	if err != nil {
		return err
	}

	params := &vshnv1.VSHNPostgreSQLParameters{
		Size:      comp.Spec.Parameters.Size,
		Instances: 1,
		Backup: vshnv1.VSHNPostgreSQLBackup{
			Retention:          retention,
			DeletionProtection: true,
		},
		Service: vshnv1.VSHNPostgreSQLServiceSpec{
			PgBouncerSettings: &sgv1.SGPoolingConfigSpecPgBouncerPgbouncerIni{
				Pgbouncer: k8sruntime.RawExtension{
					Raw: configBytes,
				},
			},
		},
	}

	if comp.Spec.Parameters.Service.PostgreSQLParameters != nil {
		err := mergo.Merge(params, comp.Spec.Parameters.Service.PostgreSQLParameters, mergo.WithOverride)
		if err != nil {
			return err
		}

		// Mergo currently has a bug with merging bools: https://github.com/darccio/mergo/issues/249
		// It's not possible to override true with false, so it won't merge this if the users disables it.
		if !comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection {
			params.Backup.DeletionProtection = false
		}
	}

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

	err = common.CreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-keycloak", svc)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(pg)
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(release)
}

func getResources(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) (common.Resources, error) {
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return common.Resources{}, err
	}

	res := common.GetResources(&comp.Spec.Parameters.Size, resources)

	return res, nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (map[string]any, error) {
	values := map[string]any{}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + pgInstanceNameSuffix)
	if err != nil {
		return nil, err
	}

	res, err := getResources(ctx, svc, comp)
	if err != nil {
		return nil, err
	}

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)
	if err != nil {
		return values, fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	values = map[string]any{
		"replicaCount": "1",
		"production":   false, // TODO: we can enable production mode with proper TLS only
		"auth": map[string]any{
			"adminUser":         "admin",
			"existingSecret":    adminSecret,
			"passwordSecretKey": adminPWSecretField,
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
		"metrics": map[string]any{
			"enabled": true,
			"serviceMonitor": map[string]any{
				"enabled": true,
			},
		},
		"httpRelativePath": comp.Spec.Parameters.Service.RelativePath,
		"postgresql": map[string]any{
			"enabled": false,
		},
		"externalDatabase": map[string]any{
			"host":     string(cd[vshnpostgres.PostgresqlHost]),
			"port":     string(cd[vshnpostgres.PostgresqlPort]),
			"database": string(cd[vshnpostgres.PostgresqlDb]),
			"user":     string(cd[vshnpostgres.PostgresqlUser]),
			"password": string(cd[vshnpostgres.PostgresqlPassword]),
		},
		"extraVolumeMounts": []map[string]string{
			{
				"name":      "postgresql-certs",
				"mountPath": "/opt/bitnami/keycloak/certs/",
			},
		},
		"extraVolumes": []map[string]any{
			{
				"name": "postgresql-certs",
				"secret": map[string]any{
					"secretName":  pgSecretName,
					"defaultMode": 420,
				},
			},
		},
		"extraEnvVars": []map[string]string{
			{
				"name":  "KEYCLOAK_JDBC_PARAMS",
				"value": "sslmode=verify-full&sslrootcert=/opt/bitnami/keycloak/certs/ca.crt",
			},
		},
	}

	fqdn := comp.Spec.Parameters.Service.FQDN
	if fqdn != "" {
		values["ingress"] = map[string]any{
			"hostname": fqdn,
		}
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (*xhelmv1.Release, error) {
	values, err := newValues(ctx, svc, comp, adminSecret)
	if err != nil {
		return nil, err
	}

	release, err := common.NewRelease(ctx, svc, comp, values)

	return release, err
}
