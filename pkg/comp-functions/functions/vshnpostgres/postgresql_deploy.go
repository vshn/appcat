package vshnpostgres

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	sgv1beta1 "github.com/vshn/appcat/v4/apis/stackgres/v1beta1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	certificateSecretName = "tls-certificate"
	namespaceResName      = "namespace-conditions"
)

//go:embed scripts/copy-pg-backup.sh
var postgresqlCopyJobScript string

func DeployPostgreSQL(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "postgresql", namespaceResName, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}
	l.Info("Set major version in status")
	err = setMajorVersionStatus(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	l.Info("Create tls certificate")
	err = createCerts(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	l.Info("Create Stackgres objects")
	err = createStackgresObjects(ctx, comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create stackgres objects: %w", err).Error())
	}

	l.Info("Create ObjectBucket")
	err = createObjectBucket(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create xObjectBucket object: %w", err).Error())
	}

	l.Info("Create SgObjectStorage")
	err = createSgObjectStorage(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create sgObjectStorage object: %w", err).Error())
	}

	l.Info("Create podMonitor")
	err = createPodMonitor(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create podMonitor object: %w", err).Error())
	}

	if comp.Spec.Parameters.Restore != nil && comp.Spec.Parameters.Restore.BackupName != "" && comp.Spec.Parameters.Restore.ClaimName != "" {
		l.Info("Create copy job")
		err = createCopyJob(comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Errorf("cannot create copyJob object: %w", err).Error())
		}
	}
	return nil
}

// setMajorVersionStatus sets version in status only when it is provisioned
// The subsequent update of this field is to happen in the MajorUpgrade comp-func
func setMajorVersionStatus(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	if comp.Status.CurrentVersion == "" {
		comp.Status.CurrentVersion = comp.Spec.Parameters.Service.MajorVersion
		return svc.SetDesiredCompositeStatus(comp)
	}
	return nil
}

func createCerts(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	selfSignedIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				SelfSigned: &cmv1.SelfSignedIssuer{
					CRLDistributionPoints: []string{},
				},
			},
		},
	}

	err := svc.SetDesiredKubeObjectWithName(selfSignedIssuer, comp.GetName()+"-localca", "local-ca", runtime.KubeOptionProtectedBy("cluster"))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	certificate := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.CertificateSpec{
			SecretName: certificateSecretName,
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat",
				},
			},
			IsCA: false,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			Usages: []cmv1.KeyUsage{"server auth", "client auth"},
			DNSNames: []string{
				comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
				comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc",
			},
			IssuerRef: certmgrv1.ObjectReference{
				Name:  comp.GetName(),
				Kind:  selfSignedIssuer.GetObjectKind().GroupVersionKind().Kind,
				Group: selfSignedIssuer.GetObjectKind().GroupVersionKind().Group,
			},
		},
	}

	err = svc.SetDesiredKubeObjectWithName(certificate, comp.GetName()+"-certificate", "certificate", runtime.KubeOptionProtectedBy("cluster"))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	return nil
}

func createStackgresObjects(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	err := createSgInstanceProfile(ctx, comp, svc)

	if err != nil {
		return err
	}

	err = createSgPostgresConfig(comp, svc)

	if err != nil {
		return err
	}

	err = createSgCluster(ctx, comp, svc)

	if err != nil {
		return err
	}

	setClusterUnreadyIfProfilesNotEqual(svc)

	return nil
}

func createSgInstanceProfile(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	l := svc.Log
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return err
	}

	containers, err := utils.FetchSidecarsFromConfig(ctx, svc)

	if err != nil {
		err = fmt.Errorf("cannot get sideCars from config: %w", err)
		return err
	}

	initContainers, err := utils.FetchInitContainersFromConfig(ctx, svc)

	if err != nil {
		err = fmt.Errorf("cannot get initContainers from config: %w", err)
		return err
	}

	sideCarMap := map[string]string{
		"createBackup":               "backup.create-backup",
		"clusterController":          "cluster-controller",
		"runDbops":                   "dbops.run-dbops",
		"setDbopsResult":             "dbops.set-dbops-result",
		"envoy":                      "envoy",
		"pgbouncer":                  "pgbouncer",
		"postgresUtil":               "postgres-util",
		"prometheusPostgresExporter": "prometheus-postgres-exporter",
	}

	initContainerMap := map[string]string{
		"pgbouncerAuthFile":          "pgbouncer-auth-file",
		"relocateBinaries":           "relocate-binaries",
		"setupScripts":               "setup-scripts",
		"setupArbitraryUser":         "setup-arbitrary-user",
		"clusterReconciliationCycle": "cluster-reconciliation-cycle",
		"setDbopsRunning":            "dbops.set-dbops-running",
		"setMajorVersionUpgrade":     "major-version-upgrade",
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) != 0 {
		l.Error(errors.Join(errs...), "Cannot get Resources from plan and claim")
	}
	containersRequests := generateContainers(*containers, sideCarMap, false)

	containersRequestsBytes, err := json.Marshal(containersRequests)
	if err != nil {
		return err
	}
	containersLimits := generateContainers(*containers, sideCarMap, true)
	containersLimitsBytes, err := json.Marshal(containersLimits)
	if err != nil {
		return err
	}

	initContainersRequests := generateContainers(*initContainers, initContainerMap, false)
	initContainersRequestsBytes, err := json.Marshal(initContainersRequests)
	if err != nil {
		return err
	}

	initContainersLimits := generateContainers(*initContainers, initContainerMap, true)
	initContainersLimitsBytes, err := json.Marshal(initContainersLimits)
	if err != nil {
		return err
	}

	sgInstanceProfile := &sgv1.SGInstanceProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: sgv1.SGInstanceProfileSpec{
			Cpu:    res.CPU.String(),
			Memory: res.Mem.String(),
			Requests: &sgv1.SGInstanceProfileSpecRequests{
				Cpu:    ptr.To(res.ReqCPU.String()),
				Memory: ptr.To(res.ReqMem.String()),
				Containers: k8sruntime.RawExtension{
					Raw: containersRequestsBytes,
				},
				InitContainers: k8sruntime.RawExtension{
					Raw: initContainersRequestsBytes,
				},
			},
			Containers: k8sruntime.RawExtension{
				Raw: containersLimitsBytes,
			},
			InitContainers: k8sruntime.RawExtension{
				Raw: initContainersLimitsBytes,
			},
		},
	}

	err = svc.SetDesiredKubeObjectWithName(sgInstanceProfile, comp.GetName()+"-profile", "profile")
	if err != nil {
		err = fmt.Errorf("cannot create sgInstanceProfile: %w", err)
		return err
	}

	return nil
}

func generateContainers(s utils.Sidecars, containerMap map[string]string, limits bool) map[string]sgv1.SGInstanceProfileContainer {

	containers := map[string]sgv1.SGInstanceProfileContainer{}

	if limits {
		for k, v := range containerMap {
			containers[v] = sgv1.SGInstanceProfileContainer{
				Cpu:    s[k].Limits.CPU,
				Memory: s[k].Limits.Memory,
			}
		}
	} else {
		for k, v := range containerMap {
			containers[v] = sgv1.SGInstanceProfileContainer{
				Cpu:    s[k].Requests.CPU,
				Memory: s[k].Requests.Memory,
			}
		}
	}

	return containers
}

func createSgPostgresConfig(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	pgConfBytes := comp.Spec.Parameters.Service.PostgreSQLSettings

	pgConf := map[string]string{}
	if pgConfBytes.Raw != nil {
		err := json.Unmarshal(pgConfBytes.Raw, &pgConf)
		if err != nil {
			return fmt.Errorf("cannot unmarshall pgConf: %w", err)
		}
	}

	pgConfigName, pgKubeName := getCurrentSettings(comp, svc, comp.Status.CurrentVersion, comp.Status.PreviousVersion)

	currentVersionConfig := sgv1.SGPostgresConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgConfigName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: sgv1.SGPostgresConfigSpec{
			PostgresVersion: comp.Status.CurrentVersion,
			PostgresqlConf:  pgConf,
		},
	}

	err := svc.SetDesiredKubeObject(&currentVersionConfig, pgKubeName, runtime.KubeOptionAllowDeletion)
	if err != nil {
		return fmt.Errorf("cannot create current version postgres config: %w", err)
	}

	return nil
}

// getCurrentSettings returns the kube object name pgKubeName and wrapped resource name pgConfigName
// This function ensures compatibility with older Postgres instances where SGPGSettings resources name was missing the major version
// If such resource exists (ex:. pg-instance-cntqx) then it will be kept with this naming until a new major version upgrade is issued
// New postgres instances have the format:
// Kube name - pg-instance-pg-conf-14
// Resource name - pg-instance-postgres-config-14
func getCurrentSettings(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, currentV, previousV string) (string, string) {
	pgConfigName := fmt.Sprintf("%s-postgres-config-%s", comp.GetName(), currentV)
	pgKubeName := fmt.Sprintf("%s-%s-%s", comp.GetName(), configResourceName, currentV)

	existingProfile := &sgv1.SGPostgresConfig{}
	_ = svc.GetObservedKubeObject(existingProfile, fmt.Sprintf("%s-%s", comp.GetName(), configResourceName))

	if existingProfile.Name != "" && previousV == "" {
		pgKubeName = fmt.Sprintf("%s-%s", comp.GetName(), configResourceName)
		pgConfigName = existingProfile.Name
	}
	return pgConfigName, pgKubeName
}

func createSgCluster(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	l := svc.Log

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) != 0 {
		l.Error(errors.Join(errs...), "Cannot get Resources from plan and claim")
	}
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)

	if err != nil {
		return fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	pgConfigName, _ := getCurrentSettings(comp, svc, comp.Status.CurrentVersion, comp.Status.PreviousVersion)

	initialData := &sgv1.SGClusterSpecInitialData{}
	backupRef := xkubev1.Reference{}
	if comp.Spec.Parameters.Restore != nil && comp.Spec.Parameters.Restore.BackupName != "" {
		initialData = &sgv1.SGClusterSpecInitialData{
			Restore: &sgv1.SGClusterSpecInitialDataRestore{
				FromBackup: &sgv1.SGClusterSpecInitialDataRestoreFromBackup{
					Name:           &comp.Spec.Parameters.Restore.BackupName,
					TargetTimeline: &comp.Spec.Parameters.Restore.RecoveryTimeStamp,
				},
			},
		}
	}

	sgCluster := &sgv1.SGCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: sgv1.SGClusterSpec{
			Instances:         comp.Spec.Parameters.Instances,
			SgInstanceProfile: ptr.To(comp.GetName()),
			Configurations: &sgv1.SGClusterSpecConfigurations{
				SgPostgresConfig: ptr.To(pgConfigName),
				Backups: &[]sgv1.SGClusterSpecConfigurationsBackupsItem{
					{
						SgObjectStorage: "sgbackup-" + comp.GetName(),
						Retention:       &comp.Spec.Parameters.Backup.Retention,
					},
				},
			},
			InitialData: initialData,
			Postgres: sgv1.SGClusterSpecPostgres{
				Version: comp.Status.CurrentVersion,
			},
			Pods: sgv1.SGClusterSpecPods{
				PersistentVolume: sgv1.SGClusterSpecPodsPersistentVolume{
					Size: res.Disk.String(),
				},
				Resources: &sgv1.SGClusterSpecPodsResources{
					EnableClusterLimitsRequirements: ptr.To(true),
				},
				Scheduling: &sgv1.SGClusterSpecPodsScheduling{
					NodeSelector: nodeSelector,
				},
				DisableConnectionPooling: ptr.To(comp.Spec.Parameters.Service.DisablePgBouncer),
				DisableEnvoy:             ptr.To(false),
			},
			NonProductionOptions: &sgv1.SGClusterSpecNonProductionOptions{
				EnableSetPatroniCpuRequests:    ptr.To(true),
				EnableSetPatroniMemoryRequests: ptr.To(true),
				EnableSetClusterCpuRequests:    ptr.To(true),
				EnableSetClusterMemoryRequests: ptr.To(true),
			},
		},
	}

	TLSSettings := &sgv1.SGClusterSpecPostgresSsl{
		Enabled: &comp.Spec.Parameters.Service.TLS.Enabled,
	}

	if comp.Spec.Parameters.Service.TLS.Enabled {
		TLSSettings.CertificateSecretKeySelector = &sgv1.SGClusterSpecPostgresSslCertificateSecretKeySelector{
			Name: certificateSecretName,
			Key:  "tls.crt",
		}
		TLSSettings.PrivateKeySecretKeySelector = &sgv1.SGClusterSpecPostgresSslPrivateKeySecretKeySelector{
			Name: certificateSecretName,
			Key:  "tls.key",
		}
	}

	sgCluster.Spec.Postgres.Ssl = TLSSettings

	configureReplication(comp, sgCluster)

	readiness := xkubev1.Readiness{
		// Not ready if `PendingRestart` but ready if `PendingRestart and PendingUpgrade`
		CelQuery: `object.status.conditions.exists(c, c.type == "PendingRestart" && c.status == "False" )
  || (object.status.conditions.exists(c, c.type == "PendingUpgrade" && c.status == "True") && object.status.conditions.exists(c, c.type == "PendingRestart" && c.status == "False" ))`,
	}

	// We need to protect the namespace, otherwise, if the namespace get deleted first during de-provisioning, it can delete objects that
	// are referenced in the kube object. This will lead to the object getting stuck indefinitely.
	err = svc.SetDesiredKubeObjectWithName(
		sgCluster,
		comp.GetName()+"-cluster",
		"cluster",
		runtime.KubeOptionAddRefs(backupRef),
		runtime.KubeOptionProtects(namespaceResName),
		runtime.KubeOptionCustomReadiness(readiness),
	)
	if err != nil {
		err = fmt.Errorf("cannot create sgCluster: %w", err)
		return err
	}

	return nil

}

func createObjectBucket(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	xObjectBucket := &appcatv1.XObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
			Labels: map[string]string{
				runtime.ProviderConfigIgnoreLabel: "true",
			},
		},
		Spec: appcatv1.XObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: fmt.Sprintf("%s-%s-%s", comp.GetName(), svc.Config.Data["bucketRegion"], "backup"),
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      "pgbucket-" + comp.GetName(),
					Namespace: svc.GetCrossplaneNamespace(),
				},
			},
		},
	}

	xObjectBucket.Spec.Parameters.BucketName = getBucketName(svc, xObjectBucket)

	err := svc.SetDesiredComposedResourceWithName(xObjectBucket, "pg-bucket")
	if err != nil {
		err = fmt.Errorf("cannot create xObjectBucket: %w", err)
		return err
	}

	return nil
}

func createSgObjectStorage(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	certificateExists := svc.WaitForObservedDependencies("sgbackup-"+comp.GetName(), "certificate")
	if !certificateExists {
		return fmt.Errorf("waiting for dependencies: certificate")
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails("pg-bucket")
	if err != nil {
		svc.Log.Info(fmt.Sprintf("pg-bucket connection details not yet available: %s", err.Error()))
	}

	bucket := &appcatv1.XObjectBucket{}
	err = svc.GetDesiredComposedResourceByName(bucket, "pg-bucket")
	if err != nil {
		svc.Log.Info(fmt.Sprintf("pg-bucket cannot be read: %s", err.Error()))
	}

	sgObjectStorage := &sgv1beta1.SGObjectStorage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sgbackup-" + comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: sgv1beta1.SGObjectStorageSpec{
			Type: "s3Compatible",
			S3Compatible: &sgv1beta1.SGObjectStorageSpecS3Compatible{
				Bucket:                    bucket.Spec.Parameters.BucketName,
				EnablePathStyleAddressing: ptr.To(true),
				Region:                    ptr.To(string(cd["AWS_REGION"])),
				Endpoint:                  ptr.To(string(cd["ENDPOINT_URL"])),
				AwsCredentials: sgv1beta1.SGObjectStorageSpecS3CompatibleAwsCredentials{
					SecretKeySelectors: sgv1beta1.SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectors{
						AccessKeyId: sgv1beta1.SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsAccessKeyId{
							Name: "pgbucket-" + comp.GetName(),
							Key:  "AWS_ACCESS_KEY_ID",
						},
						SecretAccessKey: sgv1beta1.SGObjectStorageSpecS3CompatibleAwsCredentialsSecretKeySelectorsSecretAccessKey{
							Name: "pgbucket-" + comp.GetName(),
							Key:  "AWS_SECRET_ACCESS_KEY",
						},
					},
				},
			},
		},
	}
	err = svc.SetDesiredKubeObjectWithName(sgObjectStorage, comp.GetName()+"-object-storage", "sg-backup")
	if err != nil {
		err = fmt.Errorf("cannot create sgBackup: %w", err)
		return err
	}

	return nil
}

func createPodMonitor(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	var keepMetrics []string
	err := json.Unmarshal([]byte(svc.Config.Data["keepMetrics"]), &keepMetrics)
	if err != nil {
		return fmt.Errorf("cannot unmarshall keepMetrics: %w", err)
	}

	podMonitor := &promv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgresql-podmonitor",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: promv1.PodMonitorSpec{
			PodMetricsEndpoints: []promv1.PodMetricsEndpoint{
				{
					Port: "pgexporter",
					MetricRelabelConfigs: []*promv1.RelabelConfig{
						{
							Action: "keep",
							SourceLabels: []promv1.LabelName{
								"__name__",
							},
							Regex: "(" + strings.Join(keepMetrics[:], "|") + ")",
						},
					},
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                       "StackGresCluster",
					"stackgres.io/cluster-name": comp.GetName(),
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{
					comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err = svc.SetDesiredKubeObjectWithName(podMonitor, comp.GetName()+"-podmonitor", "podmonitor")
	if err != nil {
		err = fmt.Errorf("cannot create podMonitor: %w", err)
		return err
	}
	return nil
}

func createCopyJob(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {

	copyJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-copyjob",
			Namespace: svc.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: "copyserviceaccount",
					Containers: []v1.Container{
						{
							Name:    "copyjob",
							Image:   "bitnami/kubectl:latest",
							Command: []string{"sh", "-c"},
							Args:    []string{postgresqlCopyJobScript},
							Env: []v1.EnvVar{
								{
									Name:  "CLAIM_NAMESPACE",
									Value: comp.GetClaimNamespace(),
								},
								{
									Name:  "CLAIM_NAME",
									Value: comp.Spec.Parameters.Restore.ClaimName,
								},
								{
									Name:  "BACKUP_NAME",
									Value: comp.Spec.Parameters.Restore.BackupName,
								},
								{
									Name:  "TARGET_NAMESPACE",
									Value: comp.GetInstanceNamespace(),
								},
							},
						},
					},
				},
			},
		},
	}

	err := svc.SetDesiredKubeObjectWithName(copyJob, comp.GetName()+"-copyjob", "copy-job")
	if err != nil {
		err = fmt.Errorf("cannot create copyJob: %w", err)
		return err
	}

	return nil
}

func setClusterUnreadyIfProfilesNotEqual(svc *runtime.ServiceRuntime) {
	desProfile := &sgv1.SGInstanceProfile{}
	obsProfile := &sgv1.SGInstanceProfile{}

	changed := false

	err := svc.GetObservedKubeObject(obsProfile, "profile")
	if err != nil {
		// There's no profile yet, so we can't compare
		svc.SetDesiredResourceReadiness("cluster", runtime.ResourceUnReady)
	}

	err = svc.GetDesiredKubeObject(desProfile, "profile")
	if err != nil {
		svc.Log.Error(err, "cannot get desired profile")
	}

	if (obsProfile.Spec.Cpu != desProfile.Spec.Cpu) ||
		(obsProfile.Spec.Memory != desProfile.Spec.Memory) {
		changed = true
	}

	if changed {
		svc.SetDesiredResourceReadiness("cluster", runtime.ResourceUnReady)
	}

}

func getBucketName(svc *runtime.ServiceRuntime, currentBucket *appcatv1.XObjectBucket) string {

	bucket := &appcatv1.XObjectBucket{}

	err := svc.GetObservedComposedResource(bucket, "pg-bucket")
	if err != nil {
		return currentBucket.Spec.Parameters.BucketName
	}

	return bucket.Spec.Parameters.BucketName
}
