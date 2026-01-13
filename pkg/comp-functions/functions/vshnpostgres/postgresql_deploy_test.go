package vshnpostgres

import (
	"context"
	"encoding/json"
	"fmt"
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
	assert.Nil(t, cluster.Spec.InitialData.Restore)
	assert.Equal(t, comp.GetName(), *cluster.Spec.SgInstanceProfile)
	assert.Equal(t, fmt.Sprintf("%s-postgres-config-%s", comp.GetName(), comp.Spec.Parameters.Service.MajorVersion), *cluster.Spec.Configurations.SgPostgresConfig)
	backups := *cluster.Spec.Configurations.Backups
	assert.Equal(t, "sgbackup-"+comp.GetName(), backups[0].SgObjectStorage)
	assert.Equal(t, comp.Spec.Parameters.Backup.Schedule, *(backups[0].CronSchedule))
	assert.Equal(t, comp.Spec.Parameters.Backup.Retention, *(backups[0].Retention))
	assert.Equal(t, comp.Spec.Parameters.Service.DisablePgBouncer, *cluster.Spec.Pods.DisableConnectionPooling)

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
	assert.NoError(t, svc.GetDesiredKubeObject(sgPostgresConfig, comp.GetName()+"-"+configResourceName+"-15"))
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
	assert.NoError(t, svc.GetDesiredKubeObject(sgPostgresConfig, "pgsql-gc9x4-"+configResourceName+"-15"))
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
	assert.Equal(t, comp.Spec.Parameters.Restore.RecoveryTimeStamp, *cluster.Spec.InitialData.Restore.FromBackup.PointInTimeRecovery.RestoreToTimestamp)

	copyJob := &batchv1.Job{}
	assert.NoError(t, svc.GetDesiredKubeObject(copyJob, "copy-job"))
	assert.Equal(t, "CLAIM_NAMESPACE", copyJob.Spec.Template.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, comp.GetClaimNamespace(), copyJob.Spec.Template.Spec.Containers[0].Env[0].Value)
}

func TestPostgreSqlDeployWithBackupDisabled(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/01_default.yaml")

	// Disable backups
	enabled := false
	comp.Spec.Parameters.Backup.Enabled = &enabled

	ctx := context.TODO()

	assert.Nil(t, DeployPostgreSQL(ctx, comp, svc))

	// SGCluster should not have backup configuration
	cluster := &sgv1.SGCluster{}
	assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))
	assert.Nil(t, cluster.Spec.Configurations.Backups)

	// Other resources should still be created
	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace-conditions"))

	sgInstanceProfile := &sgv1.SGInstanceProfile{}
	assert.NoError(t, svc.GetDesiredKubeObject(sgInstanceProfile, "profile"))
}

func TestQoSGuaranteedDetection(t *testing.T) {
	tests := []struct {
		name                           string
		cpu                            string
		memory                         string
		requestsCPU                    string
		requestsMemory                 string
		expectedQoSGuaranteed          bool
		expectedEnableSetPatroniCPU    bool
		expectedEnableSetPatroniMemory bool
		expectedDisableResourcesSplit  bool
	}{
		{
			name:                           "Plan only - should NOT have QoS Guaranteed",
			cpu:                            "",
			memory:                         "",
			requestsCPU:                    "",
			requestsMemory:                 "",
			expectedQoSGuaranteed:          false,
			expectedEnableSetPatroniCPU:    true,
			expectedEnableSetPatroniMemory: true,
			expectedDisableResourcesSplit:  false,
		},
		{
			name:                           "Custom equal limits and requests - SHOULD have QoS Guaranteed",
			cpu:                            "500m",
			memory:                         "2Gi",
			requestsCPU:                    "500m",
			requestsMemory:                 "2Gi",
			expectedQoSGuaranteed:          true,
			expectedEnableSetPatroniCPU:    false,
			expectedEnableSetPatroniMemory: false,
			expectedDisableResourcesSplit:  true,
		},
		{
			name:                           "Custom different limits and requests - should NOT have QoS Guaranteed",
			cpu:                            "600m",
			memory:                         "2Gi",
			requestsCPU:                    "300m",
			requestsMemory:                 "1Gi",
			expectedQoSGuaranteed:          false,
			expectedEnableSetPatroniCPU:    true,
			expectedEnableSetPatroniMemory: true,
			expectedDisableResourcesSplit:  false,
		},
		{
			name:                           "Only limits specified - should NOT have QoS Guaranteed",
			cpu:                            "500m",
			memory:                         "2Gi",
			requestsCPU:                    "",
			requestsMemory:                 "",
			expectedQoSGuaranteed:          false,
			expectedEnableSetPatroniCPU:    true,
			expectedEnableSetPatroniMemory: true,
			expectedDisableResourcesSplit:  false,
		},
		{
			name:                           "Only requests specified - should NOT have QoS Guaranteed",
			cpu:                            "",
			memory:                         "",
			requestsCPU:                    "500m",
			requestsMemory:                 "2Gi",
			expectedQoSGuaranteed:          false,
			expectedEnableSetPatroniCPU:    true,
			expectedEnableSetPatroniMemory: true,
			expectedDisableResourcesSplit:  false,
		},
		{
			name:                           "Partial specs (only CPU limit and request) - should NOT have QoS Guaranteed",
			cpu:                            "500m",
			memory:                         "",
			requestsCPU:                    "500m",
			requestsMemory:                 "",
			expectedQoSGuaranteed:          false,
			expectedEnableSetPatroniCPU:    true,
			expectedEnableSetPatroniMemory: true,
			expectedDisableResourcesSplit:  false,
		},
		{
			name:                           "Equivalent resources in different formats - SHOULD have QoS Guaranteed",
			cpu:                            "1",
			memory:                         "2Gi",
			requestsCPU:                    "1000m",
			requestsMemory:                 "2048Mi",
			expectedQoSGuaranteed:          true,
			expectedEnableSetPatroniCPU:    false,
			expectedEnableSetPatroniMemory: false,
			expectedDisableResourcesSplit:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/01_default.yaml")

			// Set custom resources
			comp.Spec.Parameters.Size.CPU = tt.cpu
			comp.Spec.Parameters.Size.Memory = tt.memory
			comp.Spec.Parameters.Size.Requests.CPU = tt.requestsCPU
			comp.Spec.Parameters.Size.Requests.Memory = tt.requestsMemory

			ctx := context.TODO()
			assert.Nil(t, DeployPostgreSQL(ctx, comp, svc))

			// Check SGCluster configuration
			cluster := &sgv1.SGCluster{}
			assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))

			assert.Equal(t, tt.expectedEnableSetPatroniCPU, *cluster.Spec.NonProductionOptions.EnableSetPatroniCpuRequests,
				"EnableSetPatroniCpuRequests mismatch")
			assert.Equal(t, tt.expectedEnableSetPatroniMemory, *cluster.Spec.NonProductionOptions.EnableSetPatroniMemoryRequests,
				"EnableSetPatroniMemoryRequests mismatch")
			assert.Equal(t, tt.expectedDisableResourcesSplit, *cluster.Spec.Pods.Resources.DisableResourcesRequestsSplitFromTotal,
				"DisableResourcesRequestsSplitFromTotal mismatch")

			// Check SGInstanceProfile to verify resources are set correctly
			sgInstanceProfile := &sgv1.SGInstanceProfile{}
			assert.NoError(t, svc.GetDesiredKubeObject(sgInstanceProfile, "profile"))

			if tt.expectedQoSGuaranteed {
				// For QoS Guaranteed, requests should equal limits
				assert.Equal(t, sgInstanceProfile.Spec.Cpu, *sgInstanceProfile.Spec.Requests.Cpu,
					"CPU requests should equal limits for QoS Guaranteed")
				assert.Equal(t, sgInstanceProfile.Spec.Memory, *sgInstanceProfile.Spec.Requests.Memory,
					"Memory requests should equal limits for QoS Guaranteed")
			}
		})
	}
}

func getPostgreSqlComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNPostgreSQL) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
