package vshnmariadb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	mariadbPort = "3306"
	mariadbUser = "root"
	extraConfig = "extra.cnf"
	vshnConfig  = "extra-vshn.cnf"
	// Default VSHN MariaDB configuration for character set and collation
	defaultVSHNSettings = "[mysqld]\ncharacter-set-server=utf8mb4\ncollation-server=utf8mb4_unicode_ci\n"
)

// DeployMariadb will deploy the objects to provision mariadb instance
func DeployMariadb(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := svc.Log

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Create credentials secret")
	fieldList := []string{"mariadb-galera-mariabackup-password", "mariadb-password", "mariadb-root-password"}
	passwordSecret, err := common.AddCredentialsSecret(comp, svc, fieldList, common.DisallowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create credentials secret; %w", err).Error())
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	svc.Log.Info("Creating main mariadb service")
	err = createMainService(comp, svc, passwordSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create service: %s", err))
	}

	// To make scaling up and down as seamless as possible we create a cert that also includes the
	// proxy dns names.
	tlsOpts := &common.TLSOptions{
		AdditionalSans: []string{
			comp.GetName(),
			fmt.Sprintf("*.%s-headless.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace()),
			fmt.Sprintf("*.%s.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace()),
			fmt.Sprintf("%s-headless.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace()),
			fmt.Sprintf("%s.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace()),
			fmt.Sprintf("mariadb.%s.svc.cluster.local", comp.GetInstanceNamespace()),
			fmt.Sprintf("mariadb.%s.svc", comp.GetInstanceNamespace()),
			fmt.Sprintf("proxysql-0.proxysqlcluster.%s.svc", comp.GetInstanceNamespace()),
			fmt.Sprintf("proxysql-1.proxysqlcluster.%s.svc", comp.GetInstanceNamespace()),
		},
	}

	l.Info("Creating tls certificate for mariadb instance")
	_, err = common.CreateTLSCerts(ctx, comp.GetInstanceNamespace(), comp.GetName(), svc, tlsOpts)

	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	l.Info("Creating helm release for mariadb instance")
	err = createObjectHelmRelease(ctx, comp, svc, passwordSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create helm release: %w", err).Error())
	}

	l.Info("Get connection details from secret")
	err = getConnectionDetails(comp, svc, passwordSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot get connection details: %w", err).Error())
	}

	l.Info("Manage MariaDB configuration settings")
	if rSecretValues := getReleaseSecretValue(svc, comp); len(rSecretValues) > 0 {
		if err = manageCustomDBSettings(svc, comp, rSecretValues); err != nil {
			return runtime.NewWarningResult(fmt.Errorf("cannot manage MariaDB settings: %w", err).Error())
		}
	}

	return nil
}

// Create the helm release for the mariadb instance
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, secretName string) error {

	values, err := newValues(ctx, svc, comp, secretName)
	if err != nil {
		return err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return fmt.Errorf("cannot get observed release values: %w", err)
	}

	versionTag, err := maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"}, comp.Spec.Parameters.Maintenance.DisableServiceMaintenance)
	if err != nil {
		return fmt.Errorf("cannot set mariadb version for release: %w", err)
	}

	// Extract and update the MariaDB version in status
	if err := updateMariaDBVersionFromTag(comp, svc, versionTag); err != nil {
		svc.Log.Error(err, "cannot update MariaDB version in status")
	}

	r, err := newRelease(ctx, svc, values, comp)
	if err != nil {
		return err
	}

	err = svc.AddObservedConnectionDetails(comp.Name + "-release")
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
}

func getConnectionDetails(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, secretName string) error {

	mariadbRootPw, err := getMariaDBRootPassword(secretName, svc)
	if err != nil {
		return err
	}

	svc.SetConnectionDetail("MARIADB_PORT", []byte(mariadbPort))
	svc.SetConnectionDetail("MARIADB_USERNAME", []byte(mariadbUser))
	svc.SetConnectionDetail("MARIADB_PASSWORD", mariadbRootPw)

	return nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB, secretName string) (map[string]interface{}, error) {
	l := svc.Log
	values := map[string]interface{}{}

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return values, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) != 0 {
		l.Error(errors.Join(errs...), "Cannot get Resources from plan and claim")
	}
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)

	if err != nil {
		return values, fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	values = map[string]interface{}{
		// Otherwise we can't use our mirror
		// https://github.com/bitnami/charts/issues/30850
		"global": map[string]any{
			"security": map[string]any{
				"allowInsecureImages": true,
			},
		},
		"existingSecret":       secretName,
		"fullnameOverride":     comp.GetName(),
		"replicaCount":         comp.GetInstances(),
		"mariadbConfiguration": defaultVSHNSettings,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"memory": res.ReqMem.String(),
				"cpu":    res.ReqCPU.String(),
			},
			"limits": map[string]interface{}{
				"memory": res.Mem.String(),
				"cpu":    res.CPU.String(),
			},
		},
		"networkPolicy": map[string]interface{}{
			"enabled": false,
		},
		"tls": map[string]interface{}{
			"enabled":            comp.Spec.Parameters.TLS.TLSEnabled,
			"certificatesSecret": "tls-server-certificate",
			"certFilename":       "tls.crt",
			"certKeyFilename":    "tls.key",
			"certCAFilename":     "ca.crt",
		},
		"persistence": map[string]interface{}{
			"size":         res.Disk.String(),
			"storageClass": comp.Spec.Parameters.StorageClass,
		},
		// We don't need the startup probe for Galera clusters, as ProxySQL
		// will check the state independetly and is usually faster than the probe.
		// Also for single instances it unnecessarily slows downt he provisioning.
		"startupProbe": map[string]interface{}{
			"enabled": false,
		},
		"metrics": map[string]interface{}{
			"enabled": true,
			"containerSecurityContext": map[string]interface{}{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"serviceMonitor": map[string]interface{}{
				"enabled": true,
			},
			"resources": common.GetBitnamiNano(),
		},
		"securityContext": map[string]interface{}{
			"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
		},
		"containerSecurityContext": map[string]interface{}{
			"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
		},
		"podSecurityContext": map[string]interface{}{
			"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
		},
		"nodeSelector": nodeSelector,
		"podLabels": map[string]string{
			"app": "mariadb",
		},
	}

	if registry := svc.Config.Data["imageRegistry"]; registry != "" {
		err := common.SetNestedObjectValue(values, []string{"global", "imageRegistry"}, registry)
		if err != nil {
			return nil, err
		}
	}

	if imageRepositoryPrefix := svc.Config.Data["imageRepositoryPrefix"]; imageRepositoryPrefix != "" {
		if err := common.SetNestedObjectValue(values, []string{"image"}, map[string]any{
			"repository": fmt.Sprintf("%s/mariadb-galera", imageRepositoryPrefix),
		}); err != nil {
			return nil, err
		}

		if err := common.SetNestedObjectValue(values, []string{"metrics", "image"}, map[string]any{
			"repository": fmt.Sprintf("%s/mysqld-exporter", imageRepositoryPrefix),
		}); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, values map[string]any, comp *vshnv1.VSHNMariaDB) (*xhelmbeta1.Release, error) {
	cd := []xhelmbeta1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-server-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[ca.crt]",
			},
			ToConnectionSecretKey:  "ca.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "sh.helm.release.v1." + comp.GetName() + "." + getReleaseVersion(svc, comp),
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[release]",
			},
			ToConnectionSecretKey:  "release",
			SkipPartOfReleaseCheck: true,
		},
	}
	rel, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-release", cd...)
	rel.Spec.ForProvider.Chart.Name = comp.GetServiceName() + "-galera"

	return rel, err
}

func getReleaseVersion(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) string {
	r := &xhelmbeta1.Release{}
	err := svc.GetObservedComposedResource(r, comp.GetName()+"-release")
	if err != nil {
		return "v1"
	} else {
		return "v" + strconv.Itoa(r.Status.AtProvider.Revision)
	}
}

// createMariaDBSettingsConfigMap creates a single config map with both VSHN defaults and user settings
func createMariaDBSettingsConfigMap(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) error {
	configData := map[string]string{
		vshnConfig: defaultVSHNSettings, // Always include VSHN defaults
	}

	// Add user settings or set to empty string to clear stale data
	// Crossplane's Kubernetes provider uses server-side apply which doesn't remove omitted keys.
	// Setting to empty string ensures stale user settings are cleared from the ConfigMap.
	if comp.Spec.Parameters.Service.MariadbSettings != "" {
		configData[extraConfig] = "[mysqld]\n" + comp.Spec.Parameters.Service.MariadbSettings
	} else {
		configData[extraConfig] = ""
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-extra-mariadb-settings",
			Namespace: comp.Status.InstanceNamespace,
		},
		Data: configData,
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-extra-mariadb-settings")
}

// manageCustomDBSettings will manage custom MariaDB Settings set by the user.
// User settings cannot override the existing MariaDB settings for Galera but can complement it.
// To manage the MariaDB settings we observe the release secret which has the Bitnami Helm default settings for Galera.
// Then we add VSHN utf8mb4 defaults, and finally the custom settings provided by the customer via include directives
func manageCustomDBSettings(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB, rSecretValues []byte) error {
	if err := createMariaDBSettingsConfigMap(svc, comp); err != nil {
		return fmt.Errorf("cannot create mariadb settings config map: %w", err)
	}

	decodedSecretValues, err := base64.StdEncoding.DecodeString(string(rSecretValues))
	if err != nil {
		return fmt.Errorf("cannot decode release secret: %w", err)
	}
	config, err := unzip(string(decodedSecretValues))
	if err != nil {
		return fmt.Errorf("cannot unzip release secret value: %w", err)
	}
	settings, err := extractDefaultSettings(config)
	if err != nil {
		return fmt.Errorf("cannot extract current default mariadb settings from release: %w", err)
	}

	hasUserSettings := comp.Spec.Parameters.Service.MariadbSettings != ""
	adjustedSettings := adjustSettings(settings, hasUserSettings)

	r := &xhelmbeta1.Release{}
	err = svc.GetDesiredComposedResourceByName(r, comp.GetName()+"-release")
	if err != nil {
		return fmt.Errorf("cannot get release from function: %w", err)
	}

	var values map[string]interface{}
	err = json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values)
	if err != nil {
		return fmt.Errorf("cannot unmarshall release values: %w", err)
	}

	if err = unstructured.SetNestedField(values, adjustedSettings, "mariadbConfiguration"); err != nil {
		return fmt.Errorf("cannot set updated mariadb settings: %w", err)
	}

	vm := []interface{}{
		corev1.VolumeMount{
			Name:      "vshn-conf",
			MountPath: "/bitnami/conf/" + vshnConfig,
			SubPath:   vshnConfig,
		},
	}

	if hasUserSettings {
		vm = append(vm, corev1.VolumeMount{
			Name:      "extra-conf",
			MountPath: "/bitnami/conf/" + extraConfig,
			SubPath:   extraConfig,
		})
	}

	if err = common.SetNestedObjectValue(values, []string{"extraVolumeMounts"}, vm); err != nil {
		return fmt.Errorf("cannot set extraVolumeMounts in release: %w", err)
	}

	v := []interface{}{
		corev1.Volume{
			Name: "vshn-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: comp.GetName() + "-extra-mariadb-settings",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  vshnConfig,
							Path: vshnConfig,
						},
					},
					DefaultMode: ptr.To(int32(420)),
				},
			},
		},
	}

	if hasUserSettings {
		v = append(v, corev1.Volume{
			Name: "extra-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: comp.GetName() + "-extra-mariadb-settings",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  extraConfig,
							Path: extraConfig,
						},
					},
					DefaultMode: ptr.To(int32(420)),
				},
			},
		})
	}

	if err = common.SetNestedObjectValue(values, []string{"extraVolumes"}, v); err != nil {
		return fmt.Errorf("cannot set extraVolumes in release: %w", err)
	}

	marshalledValues, err := json.Marshal(&values)
	if err != nil {
		return fmt.Errorf("cannot marshall release values: %w", err)
	}
	r.Spec.ForProvider.Values.Raw = marshalledValues

	return svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
}

// adjustSettings adds the VSHN config and optionally user extra configuration files to the Bitnami defaults
// Order: Bitnami defaults -> extra-vshn.cnf (VSHN utf8mb4 settings) -> extra.cnf (user settings, if present)
func adjustSettings(settings string, hasUserSettings bool) string {
	result := settings + "\n" + "!include /bitnami/conf/" + vshnConfig

	if hasUserSettings {
		result += "\n" + "!include /bitnami/conf/" + extraConfig
	}

	return result
}

// extractDefaultSettings get the default settings from the Bitnami Helm Chart
func extractDefaultSettings(releaseConfig string) (string, error) {
	var releaseConfigObj map[string]interface{}
	if err := json.Unmarshal([]byte(releaseConfig), &releaseConfigObj); err != nil {
		return "", fmt.Errorf("cannot unmarshall release config: %w", err)
	}

	defaultBitnamiSettings, _, err := unstructured.NestedString(releaseConfigObj, "chart", "values", "mariadbConfiguration")
	if err != nil {
		return "", fmt.Errorf("cannot extract default Bitnami settings: %w", err)
	}

	return defaultBitnamiSettings, nil
}

func unzip(source string) (string, error) {
	reader := bytes.NewReader([]byte(source))
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("cannot unzip source: %w", err)
	}
	output, err := io.ReadAll(gzreader)
	if err != nil {
		return "", fmt.Errorf("cannot unzip source: %w", err)
	}
	return string(output), nil
}

// getReleaseSecretValue will fetch the release default helm chart configuration from connection secret
func getReleaseSecretValue(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) []byte {
	m, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-release")
	if err != nil {
		// The release is not yet deploy, wait for the next reconciliation
		return []byte{}
	}
	return m["release"]
}

// To make scaling as seamless as possible we create an additional service.
// This service will either point to a single db instance, or to the proxysql cluster.
func createMainService(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, secretName string) error {
	target := map[string]string{}
	targetPort := 3306
	serviceName := "mariadb"

	if comp.GetInstances() == 1 {
		target["app"] = "mariadb"
	} else {
		target["app"] = "proxysql"
		targetPort = 6033
	}

	serviceType := corev1.ServiceTypeClusterIP
	if comp.Spec.Parameters.Network.ServiceType == "LoadBalancer" {
		serviceType = corev1.ServiceTypeLoadBalancer
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: target,
			Type:     serviceType,
			Ports: []corev1.ServicePort{
				{
					Name:       serviceName,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(targetPort),
					Port:       3306,
				},
			},
		},
	}

	mariadbRootPw, err := getMariaDBRootPassword(secretName, svc)
	if err != nil {
		return err
	}

	mariadbHost := serviceName + ".vshn-mariadb-" + comp.GetName() + ".svc.cluster.local"
	mariadbURL := fmt.Sprintf("mysql://%s:%s@%s:%s", mariadbUser, mariadbRootPw, mariadbHost, mariadbPort)

	svc.SetConnectionDetail("MARIADB_HOST", []byte(mariadbHost))
	svc.SetConnectionDetail("MARIADB_URL", []byte(mariadbURL))

	if serviceType == corev1.ServiceTypeLoadBalancer {
		service := &corev1.Service{}
		err := svc.GetObservedKubeObject(service, comp.GetName()+"-main-service")
		if err != nil {
			if err != runtime.ErrNotFound {
				return err
			}
		} else {
			if len(service.Status.LoadBalancer.Ingress) != 0 {
				svc.SetConnectionDetail("LOADBALANCER_IP", []byte(service.Status.LoadBalancer.Ingress[0].IP))
			}
			err := common.AddLoadbalancerNetpolicy(svc, comp)
			if err != nil {
				return err
			}
		}
	}

	return svc.SetDesiredKubeObject(&service, comp.GetName()+"-main-service")
}

func getMariaDBRootPassword(secretName string, svc *runtime.ServiceRuntime) ([]byte, error) {
	secret := &corev1.Secret{}

	err := svc.GetObservedKubeObject(secret, secretName)

	if err != nil {
		if err == runtime.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return secret.Data["mariadb-root-password"], nil
}

// updateMariaDBVersionFromTag extracts the semantic version from a tag and appends MariaDB-log suffix
func updateMariaDBVersionFromTag(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, tag string) error {
	if tag == "" {
		return nil
	}

	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) <= 1 {
		return nil
	}

	version := matches[1] + "-MariaDB-log"
	if version != comp.Status.MariaDBVersion {
		svc.Log.Info("Updating MariaDB version in status", "version", version)
		comp.Status.MariaDBVersion = version
		return svc.SetDesiredCompositeStatus(comp)
	}

	return nil
}
