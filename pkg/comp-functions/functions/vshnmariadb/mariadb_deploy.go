package vshnmariadb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"strconv"
)

const (
	mariadbPort = "3306"
	mariadbUser = "root"
	extraConfig = "extra.cnf"
)

// DeployMariadb will deploy the objects to provision mariadb instance
func DeployMariadb(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := svc.Log

	comp := &vshnv1.VSHNMariaDB{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Create credentials secret")
	fieldList := []string{"mariadb-galera-mariabackup-password", "mariadb-password", "mariadb-root-password"}
	passwordSecret, err := common.AddCredentialsSecret(comp, svc, fieldList)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create credentials secret; %w", err).Error())
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "mariadb", comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	l.Info("Creating tls certificate for mariadb instance")
	err = common.CreateTlsCerts(ctx, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
	}

	l.Info("Creating helm release for mariadb instance")
	err = createObjectHelmRelease(ctx, comp, svc, passwordSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create helm release: %w", err).Error())
	}

	l.Info("Creating network policies mariadb instance")
	sourceNs := []string{comp.GetClaimNamespace()}
	if svc.GetBoolFromCompositionConfig("slosEnabled") {
		sourceNs = append(sourceNs, svc.Config.Data["slosNs"])
	}
	err = common.CreateNetworkPolicy(sourceNs, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create helm release: %w", err).Error())
	}

	l.Info("Get connection details from secret")
	err = getConnectionDetails(comp, svc, passwordSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot get connection details: %w", err).Error())
	}

	l.Info("Observe default release secret in case of custom MariaDB settings")
	if rSecretValues := getReleaseSecretValue(svc, comp); len(rSecretValues) > 0 && comp.Spec.Parameters.Service.MariadbSettings != "" {
		if err = manageCustomDBSettings(svc, comp, rSecretValues); err != nil {
			return runtime.NewWarningResult(fmt.Errorf("cannot set custom MariaDB settings: %w", err).Error())
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

	_, err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return fmt.Errorf("cannot set mariadb version for release: %w", err)
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
	secret := &corev1.Secret{}

	err := svc.GetObservedKubeObject(secret, secretName)

	if err != nil {
		if err == runtime.ErrNotFound {
			return nil
		}
		return err
	}
	mariadbRootPw := secret.Data["mariadb-root-password"]

	mariadbHost := comp.GetName() + ".vshn-mariadb-" + comp.GetName() + ".svc.cluster.local"
	mariadbURL := fmt.Sprintf("mysql://%s:%s@%s:%s", mariadbUser, mariadbRootPw, mariadbHost, mariadbPort)

	svc.SetConnectionDetail("MARIADB_HOST", []byte(mariadbHost))
	svc.SetConnectionDetail("MARIADB_PORT", []byte(mariadbPort))
	svc.SetConnectionDetail("MARIADB_USERNAME", []byte(mariadbUser))
	svc.SetConnectionDetail("MARIADB_URL", []byte(mariadbURL))
	svc.SetConnectionDetail("MARIADB_PASSWORD", mariadbRootPw)

	return nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB, secretName string) (map[string]interface{}, error) {

	values := map[string]interface{}{}

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return values, err
	}

	res := common.GetResources(&comp.Spec.Parameters.Size, resources)
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)

	if err != nil {
		return values, fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	values = map[string]interface{}{
		"existingSecret":   secretName,
		"fullnameOverride": comp.GetName(),
		"replicaCount":     1,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"memory": res.ReqMem,
				"cpu":    res.ReqCPU,
			},
			"limits": map[string]interface{}{
				"memory": res.Mem,
				"cpu":    res.CPU,
			},
		},
		"networkPolicy": map[string]interface{}{
			"enabled": false,
		},
		"tls": map[string]interface{}{
			"enabled":            true,
			"certificatesSecret": "tls-server-certificate",
			"certFilename":       "tls.crt",
			"certKeyFilename":    "tls.key",
			"certCAFilename":     "ca.crt",
		},
		"persistence": map[string]interface{}{
			"size":         res.Disk,
			"storageClass": comp.Spec.Parameters.StorageClass,
		},
		"startupProbe": map[string]interface{}{
			"enabled": true,
		},
		"metrics": map[string]interface{}{
			"enabled": true,
			"containerSecurityContext": map[string]interface{}{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"serviceMonitor": map[string]interface{}{
				"enabled": true,
			},
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
	rel, err := common.NewRelease(ctx, svc, comp, values, cd...)
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

// createUserSettingsConfigMap create a config map from the custom customer MariaDB settings
func createUserSettingsConfigMap(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-extra-mariadb-settings",
			Namespace: comp.Status.InstanceNamespace,
		},
		Data: map[string]string{
			extraConfig: comp.Spec.Parameters.Service.MariadbSettings,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-extra-mariadb-settings")
}

// manageCustomDBSettings will manage custom MariaDB Settings set by the user.
// User settings cannot override the existing MariaDB settings for Galera but can complement it.
// To manage the MariaDB settings first we observe the release secret which has the Helm default settings for Galera.
// Then we explicitly set the default settings with the custom settings provided by the customer via include directive
func manageCustomDBSettings(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB, rSecretValues []byte) error {
	if err := createUserSettingsConfigMap(svc, comp); err != nil {
		return fmt.Errorf("cannot create mariadb user settings config map: %w", err)
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

	adjustedSettings := adjustSettings(settings)

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

	// reset default settings with include directive
	err = unstructured.SetNestedField(values, adjustedSettings, "mariadbConfiguration")
	if err != nil {
		return fmt.Errorf("cannot set updated mariadb settings: %w", err)
	}
	// set extra volume for custom user settings
	vm := []interface{}{
		corev1.VolumeMount{
			Name:      "extra-conf",
			MountPath: "/bitnami/conf/" + extraConfig,
			SubPath:   extraConfig,
		},
	}
	err = common.SetNestedObjectValue(values, []string{"extraVolumeMounts"}, vm)
	if err != nil {
		return fmt.Errorf("cannot set extraVolumeMounts in release: %w", err)
	}

	// configure extra volume for custom user settings
	v := []interface{}{
		corev1.Volume{
			Name: "extra-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: comp.GetName() + "-extra-mariadb-settings",
					},
					DefaultMode: ptr.To(int32(420)),
				},
			},
		},
	}
	err = common.SetNestedObjectValue(values, []string{"extraVolumes"}, v)
	if err != nil {
		return fmt.Errorf("cannot set extraVolumes in release: %w", err)
	}

	marshalledValues, err := json.Marshal(&values)
	if err != nil {
		return fmt.Errorf("cannot marshall release values: %w", err)
	}
	r.Spec.ForProvider.Values.Raw = marshalledValues

	err = svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
	if err != nil {
		return fmt.Errorf("cannot set desired release: %w", err)
	}
	return nil
}

// adjustSettings adds the extra configuration file to the default configuration file
func adjustSettings(settings string) string {
	return settings + "\n" + "!include /bitnami/conf/" + extraConfig
}

// extractDefaultSettings get the default settings of the Helm Chart
func extractDefaultSettings(releaseConfig string) (string, error) {
	var releaseConfigObj map[string]interface{}
	if err := json.Unmarshal([]byte(releaseConfig), &releaseConfigObj); err != nil {
		return "", fmt.Errorf("cannot unmarshall release config: %w", err)
	}

	defaultMariadbSettings, _, err := unstructured.NestedString(releaseConfigObj, "chart", "values", "mariadbConfiguration")
	if err != nil {
		return "", fmt.Errorf("cannot extract default mariadb settings: %w", err)
	}

	return defaultMariadbSettings, nil
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
