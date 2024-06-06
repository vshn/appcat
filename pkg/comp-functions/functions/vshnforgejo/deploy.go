package vshnforgejo

import (
	"context"
	"encoding/json"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	pgInstanceNameSuffix = "-pg"
	pgSecretName         = "pg-creds"
)

// DeployForgejo deploys a keycloak instance via the codecentric Helm Chart.
func DeployForgejo(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNForgejo{}
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

	svc.Log.Info("Adding forgejo release")
	err = addForgejo(ctx, svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add forgejo release: %s", err))
	}

	return nil
}

// TODO: copied from keycloak, could probably go to common
func addPostgreSQL(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) error {
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
		Size:        comp.Spec.Parameters.Size,
		Maintenance: comp.GetFullMaintenanceSchedule(),
		Backup: vshnv1.VSHNPostgreSQLBackup{
			Retention:          retention,
			DeletionProtection: ptr.To(true),
			DeletionRetention:  7,
		},
		Service: vshnv1.VSHNPostgreSQLServiceSpec{
			PgBouncerSettings: &sgv1.SGPoolingConfigSpecPgBouncerPgbouncerIni{
				Pgbouncer: k8sruntime.RawExtension{
					Raw: configBytes,
				},
			},
		},
	}

	// if comp.Spec.Parameters.Service.PostgreSQLParameters != nil {
	// 	err := mergo.Merge(params, comp.Spec.Parameters.Service.PostgreSQLParameters, mergo.WithOverride)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Mergo doesn't override non-default values with default values. So
	// 	// changing true to false is not possible with a merge.
	// 	// This is a small hack to fix this.
	// 	// `mergo.WithOverwriteWithEmptyValue` opens a new can of worms, so it's
	// 	// not used here. https://github.com/darccio/mergo/issues/249
	// 	if comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection != nil {
	// 		params.Backup.DeletionProtection = comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection
	// 	}
	// }
	// We need to set this after the merge, as the default instance count for PostgreSQL is always 1
	// and would therefore override any value we set before the merge.
	params.Instances = 1

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

	err = common.CustomCreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-forgejo", false, svc)
	if err != nil {
		return err
	}

	err = common.DisableBilling(pg.GetInstanceNamespace(), svc)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(pg)
}

func addForgejo(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) error {

	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + pgInstanceNameSuffix)
	if err != nil {
		return err
	}

	values := map[string]any{
		"gitea": map[string]any{
			"config": map[string]any{
				"APP_NAME": "Yolo Forgejo",
				"database": map[string]any{
					"DB_TYPE": "postgres",
					"HOST":    string(cd["POSTGRESQL_HOST"]),
					"NAME":    "postgres",
					"USER":    string(cd["POSTGRESQL_USER"]),
					"PASSWD":  string(cd["POSTGRESQL_PASSWORD"]),
					"SCHEMA":  "public",
				},
			},
		},
		"postgresql": map[string]any{
			"enabled": false,
		},
		"postgresql-ha": map[string]any{
			"enabled": false,
		},
		"redis-cluster": map[string]any{
			"enabled": false,
		},
	}

	release, err := common.NewRelease(ctx, svc, comp, values)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(release)
}
