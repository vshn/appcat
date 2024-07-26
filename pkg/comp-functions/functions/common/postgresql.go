package common

import (
	"dario.cat/mergo"
	"encoding/json"
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

type PostgreSQLDependencyBuilder struct {
	svc                  *runtime.ServiceRuntime
	comp                 InfoGetter
	psqlParams           *vshnv1.VSHNPostgreSQLParameters
	pgBouncerConfig      map[string]string
	timeOfDayMaintenance vshnv1.TimeOfDay
}

func NewPostgreSQLDependencyBuilder(svc *runtime.ServiceRuntime, comp InfoGetter) *PostgreSQLDependencyBuilder {
	return &PostgreSQLDependencyBuilder{
		svc:  svc,
		comp: comp,
	}
}

func (a *PostgreSQLDependencyBuilder) AddParameters(psqlParams *vshnv1.VSHNPostgreSQLParameters) *PostgreSQLDependencyBuilder {
	a.psqlParams = psqlParams
	return a
}

func (a *PostgreSQLDependencyBuilder) AddPGBouncerConfig(pgBouncerConfig map[string]string) *PostgreSQLDependencyBuilder {
	a.pgBouncerConfig = pgBouncerConfig
	return a
}

func (a *PostgreSQLDependencyBuilder) SetCustomMaintenanceSchedule(timeOfDayMaintenance vshnv1.TimeOfDay) *PostgreSQLDependencyBuilder {
	a.timeOfDayMaintenance = timeOfDayMaintenance
	return a
}

func (a *PostgreSQLDependencyBuilder) CreateDependency() error {
	// Unfortunately k8up and stackgres backups don't match up very well...
	// if no daily backup is set we just do the default.
	retention := 6
	if a.comp.GetBackupRetention().KeepDaily != 0 {
		retention = a.comp.GetBackupRetention().KeepDaily
	}

	pgBouncerRaw := k8sruntime.RawExtension{}
	if a.pgBouncerConfig != nil {
		if a.pgBouncerConfig != nil {

			pgBouncerConfigBytes, err := json.Marshal(a.pgBouncerConfig)
			if err != nil {
				return err
			}
			pgBouncerRaw = k8sruntime.RawExtension{
				Raw: pgBouncerConfigBytes,
			}
		}
	}

	params := &vshnv1.VSHNPostgreSQLParameters{
		Size:        a.comp.GetSize(),
		Maintenance: a.comp.GetFullMaintenanceSchedule(),
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
		Monitoring: a.comp.GetMonitoring(),
	}

	if a.psqlParams != nil {
		err := mergo.Merge(params, a.psqlParams, mergo.WithOverride)
		if err != nil {
			return err
		}

		// Mergo doesn't override non-default values with default values. So
		// changing true to false is not possible with a merge.
		// This is a small hack to fix this.
		// `mergo.WithOverwriteWithEmptyValue` opens a new can of worms, so it's
		// not used here. https://github.com/darccio/mergo/issues/249
		if a.psqlParams.Backup.DeletionProtection != nil {
			params.Backup.DeletionProtection = a.psqlParams.Backup.DeletionProtection
		}
	}

	if a.timeOfDayMaintenance != "" {
		params.Maintenance.TimeOfDay = a.timeOfDayMaintenance
	}
	// We need to set this after the merge, as the default instance count for PostgreSQL is always 1
	// and would therefore override any value we set before the merge.
	params.Instances = a.comp.GetInstances()

	pg := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.comp.GetName() + PgInstanceNameSuffix,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: *params,
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      PgSecretName,
					Namespace: a.comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err := CustomCreateNetworkPolicy([]string{a.comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-"+a.comp.GetServiceName(), false, a.svc)
	if err != nil {
		return err
	}

	err = DisableBilling(pg.GetInstanceNamespace(), a.svc)
	if err != nil {
		return err
	}

	return a.svc.SetDesiredComposedResource(pg)
}
