package vshnpostgrescnpg

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
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	psqlContainerRegistry = "ghcr.io/cloudnative-pg/postgresql"
	certificateSecretName = "tls-certificate"
	namespaceResName      = "namespace-conditions"
	encryptedPvcSc        = "ssd-encrypted"
)

func DeployPostgreSQL(ctx context.Context, comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) *xfnproto.Result {
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
		return runtime.NewWarningResult(fmt.Errorf("cannot set major version: %w", err).Error())
	}

	l.Info("Create tls certificate")
	err = createCerts(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	return deployPostgresSQLUsingCNPG(ctx, comp, svc)
}

// setMajorVersionStatus sets version in status only when it is provisioned
// The subsequent update of this field is to happen in the MajorUpgrade comp-func
func setMajorVersionStatus(comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) error {
	if comp.Status.CurrentVersion == "" {
		comp.Status.CurrentVersion = comp.Spec.Parameters.Service.MajorVersion
		return svc.SetDesiredCompositeStatus(comp)
	}
	return nil
}

func createCerts(comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) error {
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

	svcName := comp.GetName() + "-cluster-rw"

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
				svcName + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
				svcName + "." + comp.GetInstanceNamespace() + ".svc",
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

func createObjectBucket(comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) error {
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

func getBucketName(svc *runtime.ServiceRuntime, currentBucket *appcatv1.XObjectBucket) string {
	bucket := &appcatv1.XObjectBucket{}

	err := svc.GetObservedComposedResource(bucket, "pg-bucket")
	if err != nil {
		return currentBucket.Spec.Parameters.BucketName
	}

	return bucket.Spec.Parameters.BucketName
}

// Deploy PostgresQL using the CNPG cluster helm chart
func deployPostgresSQLUsingCNPG(ctx context.Context, comp *vshnv1.VSHNPostgreSQLCNPG, svc *runtime.ServiceRuntime) *xfnproto.Result {
	// Deploy
	values, err := createCnpgHelmValues(ctx, svc, comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create helm values: %w", err))
	}

	svc.Log.Info("Creating Helm release for CNPG PostgreSQL")
	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-cnpg")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	// Release overrides
	release.Spec.ForProvider.Chart.Repository = svc.Config.Data["cnpgClusterChartSource"]
	release.Spec.ForProvider.Chart.Version = svc.Config.Data["cnpgClusterChartVersion"]
	release.Spec.ForProvider.Chart.Name = svc.Config.Data["cnpgClusterChartName"]
	release.Spec.ResourceSpec.WriteConnectionSecretToReference = nil

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}
	return nil
}

// Generate CNPG cluster helm chart values
func createCnpgHelmValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQLCNPG) (map[string]any, error) {
	// https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/values.yaml
	values := map[string]any{
		"cluster": map[string]any{
			"instances": 1, // For the moment we only support single instance deployments
			"imageCatalogRef": map[string]string{
				"kind": "ImageCatalog",
				"name": comp.GetName() + "-cluster",
			},
			"postgresql": map[string]any{
				"parameters": map[string]any{},
			},
			"certificates": map[string]string{
				"serverCASecret":  certificateSecretName,
				"serverTLSSecret": certificateSecretName,
			},
			// The following will be overwritten by setResources() later
			"storage":    map[string]any{},
			"walStorage": map[string]any{}, // Disabled by default, but the SC gets set
			"resources": map[string]any{
				"requests": map[string]any{},
				"limits":   map[string]any{},
			},
		},
		// The name of the ImageCatalog gets autogenerated and is the same as the cluster
		"imageCatalog": map[string]any{
			"create": true,
			// Image tags: skopeo list-tags docker://ghcr.io/cloudnative-pg/postgresql
			"images": []map[string]string{
				{
					"image": getPsqlImage("17.5"),
					"major": "17",
				},
				{
					"image": getPsqlImage("16.9"),
					"major": "16",
				},
				{
					"image": getPsqlImage("15.9"),
					"major": "15",
				},
			},
		},
		"version": map[string]string{
			"postgres": comp.Spec.Parameters.Service.MajorVersion,
		},
	}

	// Encrypted PVC
	if comp.Spec.Parameters.Encryption.Enabled {
		err := common.SetNestedObjectValue(values, []string{"cluster", "storage", "storageClass"}, encryptedPvcSc)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set storageClass (normal data) for cluster: %w", err)
		}
		err = common.SetNestedObjectValue(values, []string{"cluster", "walStorage", "storageClass"}, encryptedPvcSc)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set storageClass (WAL) for cluster: %w", err)
		}
	}

	// PostgreSQLSettings
	svc.Log.Info("Setting postgresSettings")
	pgConf, err := getPgSettingsMap(comp.Spec.Parameters.Service.PostgreSQLSettings)
	if err != nil {
		return map[string]any{}, fmt.Errorf("cannot get pg settings: %w", err)
	}

	for k, v := range pgConf {
		err = common.SetNestedObjectValue(values, []string{"cluster", "postgresql", "parameters", k}, v)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set pg settings %s=%s: %w", k, v, err)
		}
	}

	// Compute resources
	svc.Log.Info("Fetching and setting compute resources")
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	res, err := getResourcesForPlan(ctx, svc, comp, plan)
	if err != nil {
		return map[string]any{}, fmt.Errorf("could not set resources: %w", err)
	}

	err = setResourcesCnpg(values, res)
	if err != nil {
		return map[string]any{}, fmt.Errorf("cannot set resources: %w", err)
	}

	return values, nil
}

// Set compute resources in the values map
func setResourcesCnpg(values map[string]any, resources common.Resources) error {
	err := common.SetNestedObjectValue(values, []string{"cluster", "resources", "limits", "cpu"}, resources.CPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "requests", "cpu"}, resources.ReqCPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "limits", "memory"}, resources.Mem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory limits: %w", err)
	}
	err = common.SetNestedObjectValue(values, []string{"cluster", "resources", "requests", "memory"}, resources.ReqMem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory requests: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"cluster", "storage", "size"}, resources.Disk.String())
	if err != nil {
		return fmt.Errorf("cannot set disk size: %w", err)
	}

	return nil
}

// Get resources for a given plan
func getResourcesForPlan(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQLCNPG, plan string) (common.Resources, error) {
	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return common.Resources{}, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		return common.Resources{}, errors.Join(errs...)
	}

	return res, nil
}

// Marshal PostgreSQLSettings into map[string]string
func getPgSettingsMap(pgSettings k8sruntime.RawExtension) (map[string]string, error) {
	pgConfBytes := pgSettings

	pgConf := map[string]string{}
	if pgConfBytes.Raw != nil {
		err := json.Unmarshal(pgConfBytes.Raw, &pgConf)
		if err != nil {
			return pgConf, fmt.Errorf("cannot unmarshal pgConf: %w", err)
		}
	}
	return pgConf, nil
}

// Get PostgresQL image for a provided version
func getPsqlImage(version string) string {
	if after, ok := strings.CutPrefix(version, ":"); ok {
		version = after
	}

	return psqlContainerRegistry + ":" + version
}
