package vshnkeycloak

import (
	"context"
	"crypto/md5"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	prom "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

const (
	// Each instance has two admin accounts by default.
	// One that's exposed to the user and one that's kept internally.
	// The internal one is used for scripts within the keycloak image to handle various configurations.
	internalAdminPWSecretField    = "internalAdminPassword"
	adminPWSecretField            = "adminPassword"
	adminPWConnectionDetailsField = "KEYCLOAK_PASSWORD"
	adminConnectionDetailsField   = "KEYCLOAK_USERNAME"
	hostConnectionDetailsField    = "KEYCLOAK_HOST"
	urlConnectionDetailsField     = "KEYCLOAK_URL"
	serviceSuffix                 = "keycloakx-http"
	pullsecretName                = "pullsecret"
	registryURL                   = "docker-registry.inventage.com:10121/keycloak-competence-center/keycloak-managed"
	providerInitName              = "copy-original-providers"
	realmInitName                 = "copy-original-realm-setup"
	customImagePullsecretName     = "customimagepullsecret"
	cdCertsSuffix                 = "-keycloakx-http-server-cert"
	customMountTypeSecret         = "secret"
	customMountTypeConfigMap      = "configMap"
)

//go:embed scripts/copy-kc-creds.sh
var keycloakCredentialsCopyJobScript string

//go:embed scripts/wait-kc-ready.sh
var keycloakWaitReadyScript string

//go:embed templates/copy-custom-files.tmpl
var keycloakCustomFilesCopyCommandTemplate string

// Folders that may not be replaced by the custom files init container
// https://www.keycloak.org/server/directory-structure#_directory_structure
var keycloakRootFolders = []string{
	"providers",
	"themes",
	"lib",
	"conf",
	"bin",
}

func IsKeycloakRootFolder(folder string) bool {
	for _, rootFolder := range keycloakRootFolders {
		if strings.EqualFold(folder, rootFolder) {
			return true
		}
	}
	return false
}

// DeployKeycloak deploys a keycloak instance via the codecentric Helm Chart.
func DeployKeycloak(ctx context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Adding postgresql instance")

	pgBuncerConfig := map[string]string{
		"ignore_startup_parameters": "extra_float_digits, search_path",
	}

	pgSecret, err := common.NewPostgreSQLDependencyBuilder(svc, comp).
		AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
		AddPGBouncerConfig(pgBuncerConfig).
		AddRestore(comp.Spec.Parameters.Restore, comp.Kind).
		SetCustomMaintenanceSchedule(comp.Spec.Parameters.Maintenance.TimeOfDay.AddTime(20 * time.Minute)).
		CreateDependency()
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create postgresql instance: %s", err))
	}

	svc.Log.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot bootstrap instance namespace: %s", err))
	}

	svc.Log.Info("Add pull secret")
	err = addPullSecret(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot deploy pull secret: %s", err))
	}

	svc.Log.Info("Checking readiness of cluster")

	resourceCDMap := map[string][]string{
		comp.GetName() + common.PgInstanceNameSuffix: {
			vshnpostgres.PostgresqlHost,
			vshnpostgres.PostgresqlPort,
			vshnpostgres.PostgresqlDb,
			vshnpostgres.PostgresqlUser,
			vshnpostgres.PostgresqlPassword,
		},
	}

	ready, err := svc.WaitForObservedDependenciesWithConnectionDetails(comp.GetName(), resourceCDMap)
	if err != nil {
		// We're returning a fatal here, so in case something is wrong we won't delete anything by mistake.
		return runtime.NewFatalResult(err)
	} else if !ready {
		return runtime.NewWarningResult("postgresql instance not yet ready")
	}

	svc.Log.Info("Adding release")

	var adminSecret string
	if comp.Spec.Parameters.Restore != nil {
		err := copyKeycloakCredentials(comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot copy keycloak secret: %s", err))
		}
	}
	adminSecret, err = common.AddCredentialsSecret(comp, svc, []string{internalAdminPWSecretField, adminPWSecretField}, common.DisallowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot generate admin secret: %s", err))
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed connection details for keycloak admin: %s", err))
	}

	svc.SetConnectionDetail(adminPWConnectionDetailsField, cd[adminPWSecretField])
	svc.SetConnectionDetail(adminConnectionDetailsField, []byte("admin"))
	svc.SetConnectionDetail(hostConnectionDetailsField, []byte(fmt.Sprintf("%s-%s.%s.svc.cluster.local", comp.GetName(), serviceSuffix, comp.GetInstanceNamespace())))

	if err := handleCustomConfig(ctx, comp, svc, adminSecret); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot handle custom config: %s", err))
	}

	if err := handleCustomMounts(ctx, comp, svc); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot handle custom mounts: %s", err))
	}

	err = addRelease(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	svc.Log.Info("Creating Keycloak TLS certs")
	// The helm chart appends `-keycloakx-http` to the http service.
	_, err = common.CreateTLSCerts(ctx, comp.GetInstanceNamespace(), comp.GetName()+"-keycloakx-http", svc, nil)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add tls certificate: %s", err))
	}

	cdObjectName := comp.GetName() + cdCertsSuffix

	err = svc.AddObservedConnectionDetails(cdObjectName)
	if err != nil {
		svc.Log.Error(err, "cannot set connection details")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set connection details: %s", err)))
	}

	err = addServiceMonitor(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add service monitor: %s", err))
	}

	return nil
}

func handleCustomConfig(ctx context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime, adminSecret string) error {
	l := svc.Log
	customConfigurationRef := comp.Spec.Parameters.Service.CustomConfigurationRef
	customEnvVariablesRef := comp.Spec.Parameters.Service.CustomEnvVariablesRef

	if customConfigurationRef == nil && customEnvVariablesRef == nil {
		return nil
	}

	configMapName := comp.GetName() + "-config-map"
	envSecretName := comp.GetName() + "-env-secret"

	var currentConfigHash, currentEnvHash string

	if customConfigurationRef != nil {
		cm := &corev1.ConfigMap{}
		obj, err := svc.CopyKubeResource(ctx, cm, configMapName, *customConfigurationRef, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
		if err != nil {
			return fmt.Errorf("cannot copy ConfigMap: %w", err)
		}
		dataBytes, err := json.Marshal(obj.(*corev1.ConfigMap).Data)
		if err != nil {
			return fmt.Errorf("cannot marshal ConfigMap data for hashing: %w", err)
		}
		currentConfigHash = fmt.Sprintf("%x", md5.Sum(dataBytes))
	}

	if customEnvVariablesRef != nil {
		sec := &corev1.Secret{}
		obj, err := svc.CopyKubeResource(ctx, sec, envSecretName, *customEnvVariablesRef, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
		if err != nil {
			return fmt.Errorf("cannot copy Secret: %w", err)
		}
		dataBytes, err := json.Marshal(obj.(*corev1.Secret).Data)
		if err != nil {
			return fmt.Errorf("cannot marshal Secret data for hashing: %w", err)
		}
		currentEnvHash = fmt.Sprintf("%x", md5.Sum(dataBytes))
	}

	lastConfigHash := comp.GetLastConfigHash()
	lastEnvHash := comp.GetLastEnvHash()

	if !configOrEnvChanged(currentConfigHash, lastConfigHash, currentEnvHash, lastEnvHash) {
		return nil
	}

	l.Info("Detected configuration change, managing job lifecycle")

	currentCombinedHash := fmt.Sprintf("%x", md5.Sum([]byte(currentConfigHash+currentEnvHash)))
	shortHash := currentCombinedHash[:8]
	jobName := fmt.Sprintf("%s-apply-cfg-%s", comp.GetName(), shortHash)

	if len(jobName) > 63 {
		maxLen := 63 - len(shortHash) - len("-apply-cfg-") - 1
		jobName = fmt.Sprintf("%s-apply-cfg-%s", comp.GetName()[:maxLen], shortHash)
	}

	observedJob := &batchv1.Job{}
	err := svc.GetObservedKubeObject(observedJob, jobName)

	if err != nil {
		l.Info("Job for new configuration not found. Creating it now", "jobName", jobName)
		applyJob := buildConfigApplyJob(comp, adminSecret, jobName)
		if err := svc.SetDesiredKubeObject(applyJob, jobName); err != nil {
			return fmt.Errorf("cannot create config apply Job: %w", err)
		}
		svc.SetDesiredResourceReadiness(jobName, runtime.ResourceUnReady)
		return nil
	}

	if err := svc.SetDesiredKubeObject(observedJob, jobName); err != nil {
		return fmt.Errorf("cannot set desired kube object for existing job: %w", err)
	}

	if observedJob.Status.Conditions != nil {
		for _, condition := range observedJob.Status.Conditions {
			isTrue := condition.Status == corev1.ConditionTrue
			if condition.Type == batchv1.JobComplete && isTrue {
				l.Info("Job has completed", "jobName", jobName)
				comp.SetLastConfigHash(currentConfigHash)
				comp.SetLastEnvHash(currentEnvHash)
				svc.SetDesiredResourceReadiness(jobName, runtime.ResourceReady)
				break
			} else if condition.Type == batchv1.JobFailed && isTrue {
				l.Error(
					fmt.Errorf("job has failed"),
					"The configuration job failed and requires manual intervention",
					"jobName", jobName, "reason", condition.Message,
				)
				svc.SetDesiredResourceReadiness(jobName, runtime.ResourceUnReady)
				break
			}
		}
	} else {
		l.Info("Job is still running, waiting for next reconciliation", "jobName", jobName)
		svc.SetDesiredResourceReadiness(jobName, runtime.ResourceUnReady)
	}

	return svc.SetDesiredCompositeStatus(comp)
}

func handleCustomMounts(ctx context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {
	if len(comp.Spec.Parameters.Service.CustomMounts) > 0 {
		for _, m := range comp.Spec.Parameters.Service.CustomMounts {
			switch m.Type {
			case customMountTypeSecret:
				obj := &corev1.Secret{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-"+m.Name, m.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return fmt.Errorf("cannot copy secret %q: %s", m.Name, err)
				}
			case customMountTypeConfigMap:
				obj := &corev1.ConfigMap{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-"+m.Name, m.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return fmt.Errorf("cannot copy configMap %q: %s", m.Name, err)
				}
			default:
				return fmt.Errorf("invalid customMount type %q for %q", m.Type, m.Name)
			}
		}
	}
	return nil
}

func configOrEnvChanged(currentConfigHash, lastConfigHash, currentEnvHash, lastEnvHash string) bool {
	configChanged := currentConfigHash != "" && currentConfigHash != lastConfigHash
	envChanged := currentEnvHash != "" && currentEnvHash != lastEnvHash
	if !configChanged && !envChanged {
		return false
	}
	return true
}

func buildConfigApplyJob(comp *vshnv1.VSHNKeycloak, adminSecret, jobName string) *batchv1.Job {
	volumes := []corev1.Volume{}
	if comp.Spec.Parameters.Service.CustomConfigurationRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "keycloak-configs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *comp.Spec.Parameters.Service.CustomConfigurationRef,
					},
				},
			},
		})
	}

	envFrom := []corev1.EnvFromSource{}
	if comp.Spec.Parameters.Service.CustomEnvVariablesRef != nil {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: *comp.Spec.Parameters.Service.CustomEnvVariablesRef,
				},
			},
		})
	}

	if comp.Spec.Parameters.Service.EnvFrom != nil {
		envFrom = append(envFrom, *comp.Spec.Parameters.Service.EnvFrom...)
	}

	KeycloakApiUrl := fmt.Sprintf("http://%s-%s.%s.svc.cluster.local", comp.GetName(), serviceSuffix, comp.GetInstanceNamespace())

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(10)),
			TTLSecondsAfterFinished: ptr.To(int32(3600)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyOnFailure,
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: pullsecretName}},
					Volumes:          volumes,
					// We need this init container as the apply-config container could otherwise start a parallel keycloak setup
					// On a fresh instance, this would lead to a race condition where the setup will abort because it found duplicate datayy
					InitContainers: []corev1.Container{
						{
							Name:            "check-keycloak-ready",
							Image:           "quay.io/curl/curl:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh", "-c", keycloakWaitReadyScript},
							Env: []corev1.EnvVar{
								{
									Name:  "KEYCLOAK_API_URL",
									Value: strings.Replace(KeycloakApiUrl, "http", "https", 1),
								},
								{
									Name:  "STALL_SECONDS",
									Value: "30",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "apply-config",
							Image:           fmt.Sprintf("%s:%s", registryURL, comp.Spec.Parameters.Service.Version),
							ImagePullPolicy: corev1.PullAlways,
							EnvFrom:         envFrom,
							Env: []corev1.EnvVar{
								{
									Name:  "KEYCLOAK_ADMIN",
									Value: "internaladmin",
								},
								{
									Name: "KEYCLOAK_ADMIN_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: adminSecret,
											},
											Key: internalAdminPWSecretField,
										},
									},
								},
								{
									Name:  "KC_HTTP_RELATIVE_PATH",
									Value: comp.Spec.Parameters.Service.RelativePath,
								},
								{
									Name:  "KEYCLOAK_API_URL",
									Value: KeycloakApiUrl,
								},
							},
							Command: []string{"sh", "-c", "/opt/keycloak/bin/keycloak-setup.sh"},
							VolumeMounts: func() []corev1.VolumeMount {
								mounts := []corev1.VolumeMount{}
								if comp.Spec.Parameters.Service.CustomConfigurationRef != nil {
									mounts = append(mounts, corev1.VolumeMount{
										Name:      "keycloak-configs",
										MountPath: "/opt/keycloak/setup/project",
									})
								}
								return mounts
							}(),
						},
					},
				},
			},
		},
	}
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret, pgSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName()+"-release")
}

func getResources(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) (common.Resources, error) {
	l := svc.Log
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return common.Resources{}, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) != 0 {
		l.Error(errors.Join(errs...), "Cannot get Resources from plan and claim")
	}

	return res, nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret, pgSecret string) (map[string]any, error) {
	values := map[string]any{}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + common.PgInstanceNameSuffix)
	if err != nil {
		return nil, err
	}

	res, err := getResources(ctx, svc, comp)
	if err != nil {
		return nil, err
	}

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)
	if err != nil {
		return values, fmt.Errorf("cannot fetch nodeSelector from the composition config: %w", err)
	}

	extraEnvMap := []map[string]any{
		{
			"name":  "KEYCLOAK_ADMIN",
			"value": "internaladmin",
		},
		{
			"name": "KEYCLOAK_ADMIN_PASSWORD",
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name": adminSecret,
					"key":  internalAdminPWSecretField,
				},
			},
		},
		{
			"name":  "KEYCLOAK_MANAGED",
			"value": "admin",
		},
		{
			"name": "KEYCLOAK_MANAGED_PASSWORD",
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name": adminSecret,
					"key":  adminPWSecretField,
				},
			},
		},
		{
			"name":  "KC_DB_URL_PROPERTIES",
			"value": "?sslmode=verify-full&sslrootcert=/certs/pg/ca.crt",
		},
		{
			"name": "JAVA_OPTS_APPEND",
			"value": `-Djava.awt.headless=true
-Djgroups.dns.query={{ include "keycloak.fullname" . }}-headless`,
		},
		{
			"name":  "KC_HTTPS_CERTIFICATE_FILE",
			"value": "/certs/keycloak/tls.crt",
		},
		{
			"name":  "KC_HTTPS_CERTIFICATE_KEY_FILE",
			"value": "/certs/keycloak/tls.key",
		},
	}

	extraEnv, err := toYAML(extraEnvMap)
	if err != nil {
		return nil, err
	}

	extraVolumesMap := []map[string]any{
		{
			"name":     "custom-providers",
			"emptyDir": nil,
		},
		{
			"name":     "custom-themes",
			"emptyDir": nil,
		},
		{
			"name":     "custom-files",
			"emtpyDir": nil,
		},
		{
			"name": "keycloak-dist",
		},
		{
			"name": "postgresql-certs",
			"secret": map[string]any{
				"secretName":  pgSecret,
				"defaultMode": 420,
			},
		},
		{
			"name": "keycloak-certs",
			"secret": map[string]any{
				"secretName":  "tls-server-certificate",
				"defaultMode": 420,
			},
		},
	}

	for _, mount := range comp.Spec.Parameters.Service.CustomMounts {
		if mount.Type == customMountTypeSecret {
			extraVolumesMap = append(extraVolumesMap, map[string]any{
				"name": "custom-secret-" + mount.Name,
				"secret": map[string]any{
					"secretName":  mount.Name,
					"defaultMode": 420,
				},
			})
		} else if mount.Type == customMountTypeConfigMap {
			extraVolumesMap = append(extraVolumesMap, map[string]any{
				"name": "custom-configmap-" + mount.Name,
				"configMap": map[string]any{
					"name":        mount.Name,
					"defaultMode": 420,
				},
			})
		}
	}

	extraVolumeMountsMap := []map[string]any{
		{
			"name":      "custom-providers",
			"mountPath": "/opt/keycloak/providers",
		},
		{
			"name":      "custom-themes",
			"mountPath": "/opt/keycloak/themes",
		},
		{
			"name":      "postgresql-certs",
			"mountPath": "/certs/pg",
		},
		{
			"name":      "keycloak-certs",
			"mountPath": "/certs/keycloak",
		},
	}

	if comp.Spec.Parameters.Service.CustomConfigurationRef != nil {
		extraVolumeMountsMap = append(extraVolumeMountsMap, map[string]any{
			"name":      "keycloak-configs",
			"mountPath": "/opt/keycloak/setup/project",
		})
	}

	// Custom file volumes and mounts
	if len(comp.Spec.Parameters.Service.CustomFiles) > 0 {
		if comp.Spec.Parameters.Service.CustomizationImage.Image == "" {
			return nil, fmt.Errorf("custom files have been defined, but no customization image")
		}

		const extraVolumeName = "customization-image-files"
		extraVolumesMap = append(extraVolumesMap, map[string]any{
			"name":     extraVolumeName,
			"emptyDir": nil,
		})

		for _, file := range comp.Spec.Parameters.Service.CustomFiles {
			destination := strings.TrimPrefix(file.Destination, "/")
			if !isCustomFileDestinationValid(destination) {
				svc.Log.Error(nil, "Custom file destination seems bad, skipping", "destination", file.Destination)
				continue
			}

			extraVolumeMountsMap = append(extraVolumeMountsMap, map[string]any{
				"name":      extraVolumeName,
				"mountPath": "/opt/keycloak/" + destination,
				"subPath":   destination,
			})
		}
	}

	for _, mount := range comp.Spec.Parameters.Service.CustomMounts {
		if mount.Type == customMountTypeSecret {
			extraVolumeMountsMap = append(extraVolumeMountsMap, map[string]any{
				"name":      "custom-secret-" + mount.Name,
				"mountPath": "/custom/secrets/" + mount.Name,
				"readOnly":  true,
			})
		} else if mount.Type == customMountTypeConfigMap {
			extraVolumeMountsMap = append(extraVolumeMountsMap, map[string]any{
				"name":      "custom-configmap-" + mount.Name,
				"mountPath": "/custom/configs/" + mount.Name,
				"readOnly":  true,
			})
		}
	}

	extraVolumes, err := toYAML(extraVolumesMap)
	if err != nil {
		return nil, err
	}
	extraVolumeMounts, err := toYAML(extraVolumeMountsMap)
	if err != nil {
		return nil, err
	}

	values = map[string]any{
		"replicas":          comp.Spec.Parameters.Instances,
		"extraEnv":          extraEnv,
		"extraVolumes":      extraVolumes,
		"extraVolumeMounts": extraVolumeMounts,

		"command": []string{
			"/opt/keycloak/bin/kc-with-setup.sh",
			"--verbose",
			"start",
			"--http-enabled=true",
			"--http-port=8080",
			"--hostname-strict=false",
			"--spi-events-listener-jboss-logging-success-level=info",
			"--spi-events-listener-jboss-logging-error-level=warn",
		},
		"database": map[string]any{
			"hostname": string(cd[vshnpostgres.PostgresqlHost]),
			"port":     string(cd[vshnpostgres.PostgresqlPort]),
			"database": string(cd[vshnpostgres.PostgresqlDb]),
			"username": string(cd[vshnpostgres.PostgresqlUser]),
			"password": string(cd[vshnpostgres.PostgresqlPassword]),
		},
		"metrics": map[string]any{
			"enabled": true,
		},
		"extraServiceMonitor": map[string]any{
			"enabled": true,
		},
		"serviceMonitor": map[string]any{
			"enabled": true,
		},
		"resources": map[string]any{
			"requests": map[string]any{
				"memory": res.ReqMem.String(),
				"cpu":    res.ReqCPU.String(),
			},
			"limits": map[string]any{
				"memory": res.Mem.String(),
				"cpu":    res.CPU.String(),
			},
		},
		"nodeSelector": nodeSelector,
		"dbchecker": map[string]any{
			"enabled":         true,
			"securityContext": nil,
		},
		// See https://github.com/keycloak/keycloak/issues/11286
		// readOnlyRootFilesystem: true
		"securityContext": map[string]any{
			"runAsUser":                nil,
			"allowPrivilegeEscalation": false,
			"capabilities": map[string]any{
				"drop": []string{
					"ALL",
				},
			},
		},
		// Workaround until https://github.com/codecentric/helm-charts/pull/784 is merged
		"livenessProbe":  "{\"httpGet\": {\"path\": \"/health/live\", \"port\": \"http-internal\", \"scheme\": \"HTTPS\"}, \"initialDelaySeconds\": 0, \"timeoutSeconds\": 5}",
		"readinessProbe": "{\"httpGet\": {\"path\": \"/health/ready\", \"port\": \"http-internal\", \"scheme\": \"HTTPS\"}, \"initialDelaySeconds\": 10, \"timeoutSeconds\": 1}",
		"startupProbe":   "{\"httpGet\": {\"path\": \"/health\", \"port\": \"http-internal\", \"scheme\": \"HTTPS\"}, \"initialDelaySeconds\": 15, \"timeoutSeconds\": 1, \"failureThreshold\": 60, \"periodSeconds\": 5}",
		"http": map[string]any{
			"relativePath": comp.Spec.Parameters.Service.RelativePath,
		},
		"podSecurityContext": nil,
	}

	var envFromMap []v1.EnvFromSource
	if comp.Spec.Parameters.Service.EnvFrom != nil {
		envFromMap = *comp.Spec.Parameters.Service.EnvFrom
	}

	// Deprecated field, the logic below is kept for backwards compatibility
	if comp.Spec.Parameters.Service.CustomEnvVariablesRef != nil {
		envFromMap = append(envFromMap, v1.EnvFromSource{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: *comp.Spec.Parameters.Service.CustomEnvVariablesRef,
				},
			},
		})
	}

	if envFromMap != nil {
		envFrom, err := toYAML(envFromMap)
		if err != nil {
			return nil, err
		}

		values["extraEnvFrom"] = envFrom
	}

	if svc.Config.Data["imageRegistry"] != "" {
		values["image"] = map[string]interface{}{
			"repository": svc.Config.Data["imageRegistry"],
		}
	}

	if busyBoxImage := svc.Config.Data["busybox_image"]; busyBoxImage != "" {
		err := common.SetNestedObjectValue(values, []string{"dbchecker", "image"}, map[string]any{
			"repository": busyBoxImage,
		})
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret, pgSecret string) (*xhelmv1.Release, error) {
	if comp.Spec.Parameters.Service.EnvFrom != nil {
		for _, r := range *comp.Spec.Parameters.Service.EnvFrom {
			if r.SecretRef != nil {
				obj := &corev1.Secret{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-env-secret-"+r.SecretRef.Name, r.SecretRef.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return nil, fmt.Errorf("cannot copy Keycloak env variable Secret to instance namespace: %w", err)
				}
			}
			if r.ConfigMapRef != nil {
				obj := &corev1.ConfigMap{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-env-cm-"+r.ConfigMapRef.Name, r.ConfigMapRef.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return nil, fmt.Errorf("cannot copy Keycloak env variable ConfigMap to instance namespace: %w", err)
				}
			}
		}
	}

	if len(comp.Spec.Parameters.Service.CustomMounts) > 0 {
		for _, m := range comp.Spec.Parameters.Service.CustomMounts {
			switch m.Type {
			case customMountTypeSecret:
				obj := &corev1.Secret{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-"+m.Name, m.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return nil, fmt.Errorf("cannot copy secret %q: %s", m.Name, err)
				}
			case customMountTypeConfigMap:
				obj := &corev1.ConfigMap{}
				_, err := svc.CopyKubeResource(ctx, obj, comp.GetName()+"-"+m.Name, m.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace())
				if err != nil {
					return nil, fmt.Errorf("cannot copy configMap %q: %s", m.Name, err)
				}
			default:
				return nil, fmt.Errorf("invalid customMount type %q for %q", m.Type, m.Name)
			}
		}
	}

	values, err := newValues(ctx, svc, comp, adminSecret, pgSecret)
	if err != nil {
		return nil, err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return nil, fmt.Errorf("cannot get observed release values: %w", err)
	}

	observedVersion, err := maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return nil, fmt.Errorf("cannot set keycloak version for release: %w", err)
	}

	err = addInitContainer(comp, values, observedVersion)
	if err != nil {
		return nil, err
	}

	err = addServiceAccount(comp, svc, values)
	if err != nil {
		return nil, err
	}

	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-release")

	release.Spec.ForProvider.Chart.Name = "keycloakx"

	return release, err
}

// Checks if a customFile.destination is not within keycloak root folder or does path traversal
func isCustomFileDestinationValid(destination string) bool {
	d := strings.TrimPrefix(destination, "./")
	d = strings.TrimPrefix(d, "/")
	return !IsKeycloakRootFolder(d) && !strings.Contains(d, "..")
}

func toYAML(obj any) (string, error) {
	yamlBytes, err := yaml.Marshal(obj)
	return string(yamlBytes), err
}

func copyCustomImagePullSecret(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {
	secretInstance := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customImagePullsecretName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	secretClaimRef := xkubev1.Reference{
		PatchesFrom: &xkubev1.PatchesFrom{
			DependsOn: xkubev1.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  comp.GetClaimNamespace(),
				Name:       comp.Spec.Parameters.Service.CustomizationImage.ImagePullSecretRef.Name,
			},
			FieldPath: ptr.To("data"),
		},
		ToFieldPath: ptr.To("data"),
	}

	return svc.SetDesiredKubeObject(secretInstance, comp.GetName()+"-custom-image-pull-secret", runtime.KubeOptionAddRefs(secretClaimRef))
}

func addPullSecret(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {

	username := svc.Config.Data["registry_username"]
	password := svc.Config.Data["registry_password"]
	auth := fmt.Sprintf("%s:%s", username, password)

	authEncoded := base64.StdEncoding.EncodeToString([]byte(auth))

	secretMap := map[string]any{
		"auths": map[string]any{
			"docker-registry.inventage.com:10121": map[string]any{
				"username": username,
				"password": password,
				"auth":     authEncoded,
			},
		},
	}

	secretJSON, err := json.Marshal(secretMap)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pullsecretName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": secretJSON,
		},
	}

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"-pull-secret")
}

func addServiceAccount(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime, values map[string]any) error {

	pullSecrets := []map[string]any{
		{
			"name": pullsecretName,
		},
	}

	if comp.Spec.Parameters.Service.CustomizationImage.ImagePullSecretRef.Name != "" {
		err := copyCustomImagePullSecret(comp, svc)
		if err != nil {
			return err
		}
		customImagePullSecret := map[string]any{
			"name": customImagePullsecretName,
		}
		pullSecrets = append(pullSecrets, customImagePullSecret)
	}

	values["serviceAccount"] = map[string]any{
		"automountServiceAccountToken": "false",
		"imagePullSecrets":             pullSecrets,
	}

	return nil
}

func addInitContainer(comp *vshnv1.VSHNKeycloak, values map[string]any, version string) error {
	extraInitContainersMap := []map[string]any{
		{
			"name":            providerInitName,
			"image":           fmt.Sprintf("%s:%s", registryURL, version),
			"imagePullPolicy": "IfNotPresent",
			"command": []string{
				"sh",
			},
			"args": []string{
				"-c",
				`echo "Copying original provider files..."
cp -R /opt/keycloak/providers/*.jar /custom-providers
ls -lh /custom-providers`,
			},
			"volumeMounts": []map[string]any{
				{
					"name":      "custom-providers",
					"mountPath": "/custom-providers",
				},
			},
		},
	}
	extraInitContainersMap, err := addCustomInitContainer(comp, extraInitContainersMap)
	if err != nil {
		return err
	}

	extraInitContainersMap, err = addCustomFileCopyInitContainer(comp, extraInitContainersMap)
	if err != nil {
		return fmt.Errorf("cannot add custom file copy init container: %w", err)
	}

	extraInitContainers, err := toYAML(extraInitContainersMap)
	if err != nil {
		return err
	}

	values["extraInitContainers"] = extraInitContainers

	return nil
}

func addCustomInitContainer(comp *vshnv1.VSHNKeycloak, extraInitContainersMap []map[string]any) ([]map[string]any, error) {
	if comp.Spec.Parameters.Service.CustomizationImage.Image != "" {

		extraInitContainersThemeProvidersMap := map[string]any{
			"name":            "copy-custom-themes-providers",
			"image":           comp.Spec.Parameters.Service.CustomizationImage.Image,
			"imagePullPolicy": "Always",
			"command": []string{
				"sh",
			},
			"args": []string{
				"-c",
				`echo "Copying custom themes..."
cp -Rv /themes/*  /custom-themes
echo "Copying custom providers..."
cp -Rv /providers/* /custom-providers
exit 0`,
			},
			"volumeMounts": []map[string]any{
				{
					"name":      "custom-providers",
					"mountPath": "/custom-providers",
				},
				{
					"name":      "custom-themes",
					"mountPath": "/custom-themes",
				},
			},
		}
		extraInitContainersMap = append(extraInitContainersMap, extraInitContainersThemeProvidersMap)
	}
	return extraInitContainersMap, nil
}

func addCustomFileCopyInitContainer(comp *vshnv1.VSHNKeycloak, extraInitContainersMap []map[string]any) ([]map[string]any, error) {
	if len(comp.Spec.Parameters.Service.CustomFiles) < 1 {
		return extraInitContainersMap, nil
	}

	files := []map[string]string{}
	for _, customFile := range comp.Spec.Parameters.Service.CustomFiles {
		finalDestination := strings.TrimPrefix(customFile.Destination, "./")
		finalDestination = strings.TrimPrefix(finalDestination, "/")
		if !isCustomFileDestinationValid(finalDestination) {
			return nil, fmt.Errorf("destination '%s' is bad", finalDestination)
		}

		files = append(files, map[string]string{
			"source":      strings.TrimPrefix(customFile.Source, "/"),
			"destination": finalDestination,
		})
	}

	var copyCommand strings.Builder
	t, err := template.New("tmpl").Funcs(map[string]any{"contains": strings.Contains}).Parse(keycloakCustomFilesCopyCommandTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	err = t.Execute(&copyCommand, files)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	extraInitContainerCustomFileCopyMap := map[string]any{
		"name":            "copy-custom-files",
		"image":           comp.Spec.Parameters.Service.CustomizationImage.Image,
		"imagePullPolicy": "Always",
		"command": []string{
			"sh",
		},
		"args": []string{
			"-c",
			copyCommand.String(),
		},
		"volumeMounts": []map[string]string{{
			"name":      "customization-image-files",
			"mountPath": "/custom",
		}},
	}
	extraInitContainersMap = append(extraInitContainersMap, extraInitContainerCustomFileCopyMap)

	return extraInitContainersMap, nil
}

func copyKeycloakCredentials(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {
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
							Args:    []string{keycloakCredentialsCopyJobScript},
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
									Name:  "TARGET_NAMESPACE",
									Value: comp.GetInstanceNamespace(),
								},
								{
									Name:  "TARGET_CLAIM",
									Value: comp.GetClaimName(),
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

func addServiceMonitor(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {

	serviceMonitor := prom.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-service-monitor",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: prom.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/component": "http",
					"app.kubernetes.io/instance":  comp.GetName(),
					"app.kubernetes.io/name":      "keycloakx",
				},
			},
			Endpoints: []prom.Endpoint{
				{
					Port:     "http-internal",
					Path:     "/metrics",
					Interval: "15s",
					Scheme:   "https",
					TLSConfig: &prom.TLSConfig{
						SafeTLSConfig: prom.SafeTLSConfig{
							InsecureSkipVerify: true,
						},
					},
				},
			},
		},
	}
	return svc.SetDesiredKubeObject(&serviceMonitor, comp.GetName()+"-service-monitor")
}
