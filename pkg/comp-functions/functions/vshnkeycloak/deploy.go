package vshnkeycloak

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
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
)

// DeployKeycloak deploys a keycloak instance via the codecentric Helm Chart.
func DeployKeycloak(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNKeycloak{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Adding postgresql instance")

	pgBuncerConfig := map[string]string{
		"ignore_startup_parameters": "extra_float_digits, search_path",
	}

	err = common.NewPostgreSQLDependencyBuilder(svc, comp).
		AddParameters(comp.Spec.Parameters.Service.PostgreSQLParameters).
		AddPGBouncerConfig(pgBuncerConfig).
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

	ready, err := svc.WaitForDependenciesWithConnectionDetails(comp.GetName(), resourceCDMap)
	if err != nil {
		// We're returning a fatal here, so in case something is wrong we won't delete anything by mistake.
		return runtime.NewFatalResult(err)
	} else if !ready {
		return runtime.NewWarningResult("postgresql instance not yet ready")
	}

	svc.Log.Info("Adding release")

	adminSecret, err := common.AddCredentialsSecret(comp, svc, []string{internalAdminPWSecretField, adminPWSecretField})
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

	err = addRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	svc.Log.Info("Creating Keycloak TLS certs")
	// The helm chart appends `-keycloakx-http` to the http service.
	err = common.CreateTlsCerts(ctx, comp.GetInstanceNamespace(), comp.GetName()+"-keycloakx-http", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add tls certificate: %s", err))
	}

	cdObjectName := comp.GetName() + cdCertsSuffix

	err = svc.AddObservedConnectionDetails(cdObjectName)
	if err != nil {
		svc.Log.Error(err, "cannot set connection details")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set connection details: %s", err)))
	}

	return nil
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret)
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

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (map[string]any, error) {
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
			"name":     "custom-setup",
			"emptyDir": nil,
		},
		{
			"name": "keycloak-dist",
		},
		{
			"name": "postgresql-certs",
			"secret": map[string]any{
				"secretName":  common.PgSecretName,
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

	if comp.Spec.Parameters.Service.CustomConfigurationRef != nil {
		extraVolumesMap = append(extraVolumesMap, map[string]any{
			"name": "keycloak-configs",
			"configMap": map[string]any{
				"name": comp.Spec.Parameters.Service.CustomConfigurationRef,
				"items": []map[string]string{
					{
						"key":  "keycloak-config.json",
						"path": "keycloak-config.json",
					},
				},
			},
		})
	}

	extraVolumes, err := toYAML(extraVolumesMap)
	if err != nil {
		return nil, err
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
			"name":      "custom-setup",
			"mountPath": "/opt/keycloak/setup",
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
			"mountPath": "/opt/keycloak/setup/keycloak-config.json",
			"subPath":   "keycloak-config.json",
		})
	}

	extraVolumeMounts, err := toYAML(extraVolumeMountsMap)
	if err != nil {
		return nil, err
	}

	envFrom := ""
	if comp.Spec.Parameters.Service.CustomEnvVariablesRef != nil {
		envFrom = "[secretRef: {name: " + *comp.Spec.Parameters.Service.CustomEnvVariablesRef + "}]"
	}

	values = map[string]any{
		"replicas":          comp.Spec.Parameters.Instances,
		"extraEnv":          extraEnv,
		"extraEnvFrom":      envFrom,
		"extraVolumes":      extraVolumes,
		"extraVolumeMounts": extraVolumeMounts,

		"command": []string{
			"/opt/keycloak/bin/kc-with-setup.sh",
			"--verbose",
			"start",
			"--http-enabled=true",
			"--http-port=8080",
			"--hostname-strict=false",
			"--hostname-strict-https=false",
			"--spi-events-listener-jboss-logging-success-level=info",
			"--spi-events-listener-jboss-logging-error-level=warn",
		},
		"image": map[string]any{
			"repository": "docker-registry.inventage.com:10121/keycloak-competence-center/keycloak-managed",
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
		"http": map[string]any{
			"relativePath": comp.Spec.Parameters.Service.RelativePath,
		},
		"podSecurityContext": nil,
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (*xhelmv1.Release, error) {

	if comp.Spec.Parameters.Service.CustomConfigurationRef != nil {
		err := copyConfigMap(comp, svc)
		if err != nil {
			return nil, fmt.Errorf("cannot copy keycloak config configmap to instance namespace: %w", err)
		}
	}
	if comp.Spec.Parameters.Service.CustomEnvVariablesRef != nil {
		err := copySecret(comp, svc)
		if err != nil {
			return nil, fmt.Errorf("cannot copy keycloak env variable secret to instance namespace: %w", err)
		}
	}

	values, err := newValues(ctx, svc, comp, adminSecret)
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

	release, err := common.NewRelease(ctx, svc, comp, values)

	release.Spec.ForProvider.Chart.Name = "keycloakx"

	return release, err
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
				Namespace:  comp.Spec.Parameters.Service.CustomizationImage.ImagePullSecretRef.DeepCopy().Namespace,
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
		{
			"name":            realmInitName,
			"image":           fmt.Sprintf("%s:%s", registryURL, version),
			"imagePullPolicy": "IfNotPresent",
			"command": []string{
				"sh",
			},
			"args": []string{
				"-c",
				`echo "Copying original setup files..."
cp -R /opt/keycloak/setup/*.json /custom-setup
ls -lh /custom-setup`,
			},
			"volumeMounts": []map[string]any{
				{
					"name":      "custom-setup",
					"mountPath": "/custom-setup",
				},
			},
		},
	}
	extraInitContainersMap, err := addCustomInitContainer(comp, extraInitContainersMap)
	if err != nil {
		return err
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

func copyConfigMap(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {

	cmObjectName := comp.GetName() + "-config-claim-observer"

	cmClaimObserver := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *comp.Spec.Parameters.Service.CustomConfigurationRef,
			Namespace: comp.GetClaimNamespace(),
		},
	}
	err := svc.SetDesiredKubeObserveObject(cmClaimObserver, cmObjectName)

	if err != nil {
		svc.Log.Info("Can't observe Keycloak config Configmap")
		svc.AddResult(runtime.NewFatalResult(fmt.Errorf("cannot get configMap: %w", err)))
	}

	configMapClaim := &corev1.ConfigMap{}
	err = svc.GetObservedKubeObject(configMapClaim, cmObjectName)
	if err != nil {
		svc.Log.Info("cannot get observed Keycloak config map")
		svc.AddResult(runtime.NewWarningResult("cannot get observed Keycloak config map"))
	}

	configMapInstance := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *comp.Spec.Parameters.Service.CustomConfigurationRef,
			Namespace:   comp.GetInstanceNamespace(),
			Labels:      configMapClaim.Labels,
			Annotations: configMapClaim.Annotations,
		},
		Data: configMapClaim.Data,
	}

	return svc.SetDesiredKubeObject(configMapInstance, comp.GetName()+"-config-map")
}

func copySecret(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) error {

	secretObjectName := comp.GetName() + "-env-secret-claim-observer"

	secretClaimObserver := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *comp.Spec.Parameters.Service.CustomEnvVariablesRef,
			Namespace: comp.GetClaimNamespace(),
		},
	}
	err := svc.SetDesiredKubeObserveObject(secretClaimObserver, secretObjectName)

	if err != nil {
		svc.Log.Info("Can't observe Keycloak env variable secret")
		svc.AddResult(runtime.NewFatalResult(fmt.Errorf("cannot get secret: %w", err)))
	}

	secretClaim := &corev1.Secret{}
	err = svc.GetObservedKubeObject(secretClaim, secretObjectName)
	if err != nil {
		svc.Log.Info("cannot get observed Keycloak env variable secret")
		svc.AddResult(runtime.NewWarningResult("cannot get observed Keycloak env variable secret"))
	}

	secretInstance := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *comp.Spec.Parameters.Service.CustomEnvVariablesRef,
			Namespace:   comp.GetInstanceNamespace(),
			Labels:      secretClaim.Labels,
			Annotations: secretClaim.Annotations,
		},
		Data: secretClaim.Data,
	}

	return svc.SetDesiredKubeObject(secretInstance, comp.GetName()+"-env-secret")
}
