package common

import (
	"encoding/json"

	"dario.cat/mergo"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	"k8s.io/utils/ptr"
)

const (
	PgInstanceNameSuffix = "-pg"
	PgSecretName         = "pg-creds"
)

func AddPostgreSQL(svc *runtime.ServiceRuntime, comp InfoGetter, psqlParams *vshnv1.VSHNPostgreSQLParameters, pgBouncerConfig map[string]string) error {
	// Unfortunately k8up and stackgres backups don't match up very well...
	// if no daily backup is set we just do the default.
	retention := 6
	if comp.GetBackupRetention().KeepDaily != 0 {
		retention = comp.GetBackupRetention().KeepDaily
	}

	pgBouncerRaw := k8sruntime.RawExtension{}
	if pgBouncerConfig != nil {

		pgBouncerConfigBytes, err := json.Marshal(pgBouncerConfig)
		if err != nil {
			return err
		}
		pgBouncerRaw = k8sruntime.RawExtension{
			Raw: pgBouncerConfigBytes,
		}
	}

	params := &vshnv1.VSHNPostgreSQLParameters{
		Size:        comp.GetSize(),
		Maintenance: comp.GetFullMaintenanceSchedule(),
		Backup: vshnv1.VSHNPostgreSQLBackup{
			Retention:          retention,
			DeletionProtection: ptr.To(true),
			DeletionRetention:  7,
		},
		Service: vshnv1.VSHNPostgreSQLServiceSpec{
			PgBouncerSettings: &sgv1.SGPoolingConfigSpecPgBouncerPgbouncerIni{
				Pgbouncer: pgBouncerRaw,
			},
		},
		Monitoring: comp.GetMonitoring(),
	}

	if psqlParams != nil {
		err := mergo.Merge(params, psqlParams, mergo.WithOverride)
		if err != nil {
			return err
		}

		// Mergo doesn't override non-default values with default values. So
		// changing true to false is not possible with a merge.
		// This is a small hack to fix this.
		// `mergo.WithOverwriteWithEmptyValue` opens a new can of worms, so it's
		// not used here. https://github.com/darccio/mergo/issues/249
		if psqlParams.Backup.DeletionProtection != nil {
			params.Backup.DeletionProtection = psqlParams.Backup.DeletionProtection
		}
	}
	// We need to set this after the merge, as the default instance count for PostgreSQL is always 1
	// and would therefore override any value we set before the merge.
	params.Instances = comp.GetInstances()

	pg := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + PgInstanceNameSuffix,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: *params,
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      PgSecretName,
					Namespace: comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err := CustomCreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-"+comp.GetServiceName(), false, svc)
	if err != nil {
		return err
	}

	err = DisableBilling(pg.GetInstanceNamespace(), svc)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(pg)
}
