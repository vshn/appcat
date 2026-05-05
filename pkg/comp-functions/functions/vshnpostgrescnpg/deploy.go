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
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"github.com/vshn/appcat/v4/pkg/controller/webhooks"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PsqlContainerRegistry = "ghcr.io/cloudnative-pg/postgresql"
	certificateSecretName = "tls-certificate"
	namespaceResName      = "namespace-conditions"
	encryptedPvcSc        = "ssd-encrypted"
)

func DeployPostgreSQL(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	l.Info("Deploying CNPG PostgresQL...")

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "postgresql", namespaceResName, svc, map[string]string{
		webhooks.ProtectionOverrideLabelStorage: "true",
	})
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	l.Info("Create tls certificate")
	err = createCerts(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	l.Info("Creating SCC role binding for OpenShift")
	err = createCnpgSCCRoleBinding(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create SCC role binding: %w", err).Error())
	}

	return deployPostgresSQLUsingCNPG(ctx, comp, svc)
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

	// KubeOptionProtectedBy will set to the helm release which is comp.GetName()
	protectedBy := comp.GetName()

	err := svc.SetDesiredKubeObjectWithName(selfSignedIssuer, comp.GetName()+"-localca", "local-ca", runtime.KubeOptionProtectedBy(protectedBy))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	svcName := "postgresql-rw"

	// Once the loadbalancer ip or gateway domain is in the connection details, we can use it to fix the tls cert
	ipAddresses := []string{}
	dnsNames := []string{}
	cd := svc.GetObservedConnectionDetails()
	if v, ok := cd["LOADBALANCER_IP"]; ok && comp.Spec.Parameters.Network.ServiceType == string(corev1.ServiceTypeLoadBalancer) && externalAccessEnabled(svc) {
		ipAddresses = append(ipAddresses, string(v))
	}
	if v, ok := cd["POSTGRESQL_GATEWAY_HOST"]; ok && comp.Spec.Parameters.Network.ServiceType == tcproute.ServiceTypeTCPGateway && externalAccessEnabled(svc) {
		dnsNames = append(dnsNames, string(v))
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
			DNSNames: append([]string{
				svcName + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
				svcName + "." + comp.GetInstanceNamespace() + ".svc",
			}, dnsNames...),
			IPAddresses: ipAddresses,
			IssuerRef: certmgrv1.ObjectReference{
				Name:  comp.GetName(),
				Kind:  selfSignedIssuer.GetObjectKind().GroupVersionKind().Kind,
				Group: selfSignedIssuer.GetObjectKind().GroupVersionKind().Group,
			},
		},
	}

	err = svc.SetDesiredKubeObjectWithName(certificate, comp.GetName()+"-certificate", "certificate", runtime.KubeOptionProtectedBy(protectedBy))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	// We slow down the readiness of the certificate so Crossplane can pick up the changes in the
	// certificate secret, before everything goes synced/ready.
	// We look at the actual source certificate and verify the expected SAN is present.
	// This ensures that we don't have to wait ~1h before the correct connectionDetails
	// are populated.
	if comp.Spec.Parameters.Network.ServiceType == string(corev1.ServiceTypeLoadBalancer) && externalAccessEnabled(svc) {
		if v, ok := cd["LOADBALANCER_IP"]; ok {
			if err := waitForCertSAN(svc, comp.GetName(), comp.GetInstanceNamespace(), certHasIP(string(v))); err != nil {
				return err
			}
		} else {
			svc.SetDesiredResourceReadiness("certificate", runtime.ResourceUnReady)
		}
	}

	if comp.Spec.Parameters.Network.ServiceType == tcproute.ServiceTypeTCPGateway && externalAccessEnabled(svc) {
		if v, ok := cd["POSTGRESQL_GATEWAY_HOST"]; ok && string(v) != "" {
			if err := waitForCertSAN(svc, comp.GetName(), comp.GetInstanceNamespace(), certHasDNS(string(v))); err != nil {
				return err
			}
		} else {
			svc.SetDesiredResourceReadiness("certificate", runtime.ResourceUnReady)
		}
	}

	return nil
}

// Deploy PostgresQL using the CNPG cluster helm chart
func deployPostgresSQLUsingCNPG(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	values, err := createCnpgHelmValues(ctx, svc, comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create helm values: %w", err))
	}

	// Handle restore before backup setup — may skip Helm release creation in the first reconciliation loop
	result, skipRelease := handleRestore(ctx, comp, svc, values)
	if skipRelease {
		return result
	}

	if err := SetupBackup(ctx, svc, comp, values); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set up backup: %v", err))
	}

	// Connection details
	connectionDetails := generateConnectionDetailInfoForRelease(comp, svc)

	svc.Log.Info("Creating Helm release for CNPG PostgreSQL")
	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-cnpg", connectionDetails...)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	// Release overrides
	release.Spec.ForProvider.Chart.Repository = svc.Config.Data["cnpgClusterChartSource"]
	release.Spec.ForProvider.Chart.Version = svc.Config.Data["cnpgClusterChartVersion"]
	release.Spec.ForProvider.Chart.Name = svc.Config.Data["cnpgClusterChartName"]

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}

	return nil
}

// Generate CNPG cluster helm chart values
func createCnpgHelmValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (map[string]any, error) {
	// https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/values.yaml

	// Handle hibernation for instances=0
	// CNPG doesn't support instances=0, use hibernation annotation instead
	instances := comp.Spec.Parameters.Instances
	hibernation := "off"
	if instances == 0 {
		instances = 1 // CNPG requires at least 1 instance
		hibernation = "on"
	}

	majorVersion := comp.Spec.Parameters.Service.MajorVersion
	pinImageTag := comp.Spec.Parameters.Maintenance.PinImageTag

	// Use the major version tag by default (e.g. ":17"), or the pinned tag if set
	imageTag := majorVersion
	if pinImageTag != "" {
		imageTag = pinImageTag
		svc.Log.Info("Using pinned image tag for PostgreSQL", "majorVersion", majorVersion, "pinnedTag", pinImageTag)
	}

	if imageTag != "" {
		comp.Status.CurrentVersion = imageTag
		if err := svc.SetDesiredCompositeStatus(comp); err != nil {
			svc.Log.Error(err, "cannot update CurrentVersion in status")
		}
	}

	// Build the single-entry ImageCatalog for the cluster's major version
	imageCatalogImages := []map[string]string{
		{
			"image": getPsqlImage(imageTag),
			"major": majorVersion,
		},
	}

	values := map[string]any{
		"fullnameOverride": "postgresql",
		"cluster": map[string]any{
			"instances": instances,
			"enablePDB": instances > 1,
			"annotations": map[string]string{
				"cnpg.io/hibernation": hibernation,
			},
			"imageCatalogRef": map[string]string{
				"kind": "ImageCatalog",
				"name": "postgresql",
			},
			"monitoring": map[string]any{
				"enabled": true,
				"prometheusRule": map[string]bool{
					"enabled": false,
				},
			},
			"postgresql": map[string]any{
				"parameters": map[string]any{},
			},
			"certificates": map[string]string{
				"serverCASecret":  certificateSecretName,
				"serverTLSSecret": certificateSecretName,
			},
			// The following will be overwritten by setResources() later
			"storage": map[string]any{},
			"resources": map[string]any{
				"requests": map[string]any{},
				"limits":   map[string]any{},
			},
		},
		// The name of the ImageCatalog gets autogenerated and is the same as the cluster
		"imageCatalog": map[string]any{
			"create": true,
			// Image tags: skopeo list-tags docker://ghcr.io/cloudnative-pg/postgresql
			"images": imageCatalogImages,
		},
		"version": map[string]string{
			"postgresql": majorVersion,
		},
	}

	// Encrypted PVC
	if comp.Spec.Parameters.Encryption.Enabled {
		err := common.SetNestedObjectValue(values, []string{"cluster", "storage", "storageClass"}, encryptedPvcSc)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set storageClass (normal data) for cluster: %w", err)
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

	// Extensions
	extensions := buildCNPGExtensionValues(comp.Spec.Parameters.Service.Extensions)
	if len(extensions) > 0 {
		err = common.SetNestedObjectValue(values, []string{"cluster", "postgresql", "extensions"}, extensions)
		if err != nil {
			return map[string]any{}, fmt.Errorf("cannot set extensions: %w", err)
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

	// User management: inject roles into Cluster spec and databases as separate CRDs
	if err := addUserManagementValues(comp, values); err != nil {
		return map[string]any{}, fmt.Errorf("cannot add user management values: %w", err)
	}

	// LoadBalancer: Inject loadbalancer config if enabled
	if err := addLoadbalancerConfig(svc, comp, values); err != nil {
		return map[string]any{}, fmt.Errorf("cannot add loadbalancer service values to the chart: %w", err)
	}

	return values, nil
}

// addUserManagementValues injects managed roles (into the Cluster spec) and databases
// (as postgresql.cnpg.io/v1 Database CRDs via the chart's databases.yaml template)
// into the Helm values, based on the access list in the composite spec.
func addUserManagementValues(comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	access := comp.Spec.Parameters.Service.Access
	if len(access) == 0 {
		return nil
	}

	roles := []map[string]any{}
	seenDatabases := map[string]bool{}
	databases := []map[string]any{}

	for _, a := range access {
		username := *a.User
		secretName := userpassSecretName(comp.GetName(), username)

		roles = append(roles, map[string]any{
			"name":   username,
			"login":  true,
			"ensure": "present",
			"passwordSecret": map[string]any{
				"name": secretName,
			},
		})

		dbname := username
		if a.Database != nil {
			dbname = *a.Database
		}

		if !seenDatabases[dbname] {
			seenDatabases[dbname] = true
			databases = append(databases, map[string]any{
				"name":                  dbname,
				"owner":                 username,
				"ensure":                "present",
				"databaseReclaimPolicy": "retain",
			})
		}
	}

	if err := common.SetNestedObjectValue(values, []string{"cluster", "roles"}, roles); err != nil {
		return fmt.Errorf("cannot set cluster roles: %w", err)
	}
	if err := common.SetNestedObjectValue(values, []string{"databases"}, databases); err != nil {
		return fmt.Errorf("cannot set databases: %w", err)
	}

	return nil
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
func getResourcesForPlan(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, plan string) (common.Resources, error) {
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

// buildCNPGExtensionValues converts the user-facing extension spec into the Helm chart values
func buildCNPGExtensionValues(extensions []vshnv1.VSHNDBaaSPostgresExtension) []map[string]any {
	result := []map[string]any{}
	for _, ext := range extensions {
		if ext.Image == "" {
			continue
		}
		imageMap := map[string]any{
			"reference": ext.Image,
		}
		if ext.ImagePullPolicy != "" {
			imageMap["pullPolicy"] = ext.ImagePullPolicy
		}

		result = append(result, map[string]any{
			"name":                   ext.Name,
			"image":                  imageMap,
			"extension_control_path": ext.ExtensionControlPath,
			"dynamic_library_path":   ext.DynamicLibraryPath,
			"ld_library_path":        ext.LdLibraryPath,
		})
	}
	return result
}

// Get PostgresQL image for a provided version
func getPsqlImage(version string) string {
	if after, ok := strings.CutPrefix(version, ":"); ok {
		version = after
	}

	return PsqlContainerRegistry + ":" + version
}

// createCnpgSCCRoleBinding binds the appcat-scc ClusterRole to the CNPG pod
func createCnpgSCCRoleBinding(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	if !svc.GetBoolFromCompositionConfig("isOpenshift") {
		return nil
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "appcat-scc",
			Namespace: comp.GetInstanceNamespace(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "postgresql",
				Namespace: comp.GetInstanceNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "appcat-scc",
		},
	}
	return svc.SetDesiredKubeObject(rb, comp.GetName()+"-scc-rb")
}

func addLoadbalancerConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {

	if !externalAccessEnabled(svc) {
		return nil
	}

	if comp.Spec.Parameters.Network.ServiceType == string(corev1.ServiceTypeLoadBalancer) {
		annotations := map[string]string{}
		if svc.Config.Data["loadbalancerAnnotations"] != "" {

			err := yaml.Unmarshal([]byte(svc.Config.Data["loadbalancerAnnotations"]), &annotations)
			if err != nil {
				svc.Log.Error(err, "cannot unmarshal loadbalancer annotations from input")
				svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal loadbalancer annotations from input: %s", err)))
			}
		}

		services := map[string]any{
			"additional": []map[string]any{
				{
					"selectorType": "rw",
					"serviceTemplate": map[string]any{
						"metadata": map[string]any{
							"name":        "primary",
							"annotations": annotations,
						},
						"spec": map[string]any{
							"type": corev1.ServiceTypeLoadBalancer,
						},
					},
				},
			},
		}

		err := common.AddLoadbalancerNetpolicy(svc, comp)
		if err != nil {
			return err
		}

		return common.SetNestedObjectValue(values, []string{"cluster", "services"}, services)
	}

	return nil
}

func externalAccessEnabled(svc *runtime.ServiceRuntime) bool {
	return svc.GetBoolFromCompositionConfig("externalDatabaseConnectionsEnabled")
}
