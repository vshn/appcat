package vshnpostgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func TestPostgreSqlDeploy(t *testing.T) {

	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/01_default.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployPostgreSQL(ctx, &vshnv1.VSHNPostgreSQL{}, svc))
	assert.Nil(t, addSchedules(ctx, &vshnv1.VSHNPostgreSQL{}, svc))
	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))
	assert.Equal(t, string("vshn"), ns.GetLabels()[utils.OrgLabelName])
	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	selfSignedIssuer := &cmv1.Issuer{}
	assert.NoError(t, svc.GetDesiredKubeObject(selfSignedIssuer, "local-ca"))
	assert.Equal(t, comp.GetName(), selfSignedIssuer.Name)
	assert.Equal(t, cmv1.SelfSignedIssuer{}, *selfSignedIssuer.Spec.IssuerConfig.SelfSigned)

	certificate := &cmv1.Certificate{}
	assert.NoError(t, svc.GetDesiredKubeObject(certificate, "certificate"))
	assert.Equal(t, comp.GetName(), certificate.Name)
	assert.Equal(t, "tls-certificate", certificate.Spec.SecretName)
	assert.Equal(t, "vshn-appcat", certificate.Spec.Subject.Organizations[0])
	dnsNames := []string{
		comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
		comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc",
	}
	assert.Equal(t, dnsNames, certificate.Spec.DNSNames)
	assert.Equal(t, time.Duration(87600*time.Hour), certificate.Spec.Duration.Duration)
	assert.Equal(t, time.Duration(2400*time.Hour), certificate.Spec.RenewBefore.Duration)
	assert.Equal(t, comp.GetName(), certificate.Spec.IssuerRef.Name)
	assert.Equal(t, selfSignedIssuer.GetObjectKind().GroupVersionKind().Kind, certificate.Spec.IssuerRef.Kind)
	assert.Equal(t, selfSignedIssuer.GetObjectKind().GroupVersionKind().Group, certificate.Spec.IssuerRef.Group)

	cluster := &sgv1.SGCluster{}
	assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))
	assert.Equal(t, comp.GetName(), cluster.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), cluster.Namespace)
	assert.Equal(t, comp.Spec.Parameters.Instances, cluster.Spec.Instances)
	assert.Equal(t, comp.Spec.Parameters.Service.MajorVersion, cluster.Spec.Postgres.Version)
	assert.Nil(t, cluster.Spec.InitialData.Restore)
	assert.Equal(t, comp.GetName(), *cluster.Spec.SgInstanceProfile)
	assert.Equal(t, comp.GetName(), *cluster.Spec.Configurations.SgPostgresConfig)
	backups := *cluster.Spec.Configurations.Backups
	assert.Equal(t, "sgbackup-"+comp.GetName(), backups[0].SgObjectStorage)
	assert.Equal(t, comp.Spec.Parameters.Backup.Schedule, *(backups[0].CronSchedule))
	assert.Equal(t, comp.Spec.Parameters.Backup.Retention, *(backups[0].Retention))

	sgInstanceProfile := &sgv1.SGInstanceProfile{}
	assert.NoError(t, svc.GetDesiredKubeObject(sgInstanceProfile, "profile"))
	assert.Equal(t, comp.GetName(), sgInstanceProfile.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), sgInstanceProfile.Namespace)
	assert.Equal(t, "250m", sgInstanceProfile.Spec.Cpu)
	assert.Equal(t, "1Gi", sgInstanceProfile.Spec.Memory)
	containers := map[string]sgv1.SGInstanceProfileContainer{}
	assert.NoError(t, json.Unmarshal(sgInstanceProfile.Spec.Containers.Raw, &containers))
	assert.Contains(t, containers, "cluster-controller")
	assert.Equal(t, "32m", containers["cluster-controller"].Cpu)
	assert.Equal(t, "256Mi", containers["cluster-controller"].Memory)
	assert.Nil(t, sgInstanceProfile.Spec.HugePages)

	sgPostgresConfig := &sgv1.SGPostgresConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(sgPostgresConfig, "pg-conf"))
	assert.Equal(t, comp.Spec.Parameters.Service.MajorVersion, sgPostgresConfig.Spec.PostgresVersion)
	assert.Equal(t, map[string]string{}, sgPostgresConfig.Spec.PostgresqlConf)

	podMonitor := &promv1.PodMonitor{}
	assert.NoError(t, svc.GetDesiredKubeObject(podMonitor, "podmonitor"))
	assert.Contains(t, podMonitor.Spec.Selector.MatchLabels, "stackgres.io/cluster-name")
	assert.Equal(t, comp.GetName(), podMonitor.Spec.Selector.MatchLabels["stackgres.io/cluster-name"])
	assert.Equal(t, "pgexporter", podMonitor.Spec.PodMetricsEndpoints[0].Port)

}

func TestPostgreSqlDeployWithPgConfig(t *testing.T) {

	svc, _ := getPostgreSqlComp(t, "vshn-postgres/deploy/02_with_pg_config.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployPostgreSQL(ctx, &vshnv1.VSHNPostgreSQL{}, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))
	assert.Equal(t, string("vshn"), ns.GetLabels()[utils.OrgLabelName])

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	cluster := &sgv1.SGCluster{}
	assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))

	sgPostgresConfig := &sgv1.SGPostgresConfig{}
	assert.NoError(t, svc.GetDesiredKubeObject(sgPostgresConfig, "pg-conf"))
	assert.Contains(t, sgPostgresConfig.Spec.PostgresqlConf, "timezone")
	assert.Equal(t, "Europe/Zurich", sgPostgresConfig.Spec.PostgresqlConf["timezone"])
}

func TestPostgreSqlDeployWithRestore(t *testing.T) {

	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/03_with_restore.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployPostgreSQL(ctx, &vshnv1.VSHNPostgreSQL{}, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))
	assert.Equal(t, string("vshn"), ns.GetLabels()[utils.OrgLabelName])

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(roleBinding, "namespace-permissions"))

	cluster := &sgv1.SGCluster{}
	assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))
	assert.Equal(t, comp.Spec.Parameters.Restore.BackupName, *cluster.Spec.InitialData.Restore.FromBackup.Name)

	copyJob := &batchv1.Job{}
	assert.NoError(t, svc.GetDesiredKubeObject(copyJob, "copy-job"))
	assert.Equal(t, "CLAIM_NAMESPACE", copyJob.Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, comp.GetClaimNamespace(), copyJob.Spec.Template.Spec.Containers[0].Env[0].Value)
}

func getPostgreSqlComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNPostgreSQL) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
