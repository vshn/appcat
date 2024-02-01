package vshnkeycloak

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"dario.cat/mergo"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	pgInstanceNameSuffix          = "-pg"
	pgSecretName                  = "pg-creds"
	adminPWSecretField            = "password"
	adminPWConnectionDetailsField = "KEYCLOAK_PASSWORD"
	adminConnectionDetailsField   = "KEYCLOAK_USERNAME"
	hostConnectionDetailsField    = "KEYCLOAK_HOST"
	urlConnectionDetailsField     = "KEYCLOAK_URL"
	serviceSuffix                 = "keycloakx-http"
	pullsecretName                = "pullsecret"
	registryURL                   = "docker-registry.inventage.com:10121/keycloak-competence-center/keycloak-managed"
	providerInitName              = "copy-original-providers"
	realmInitName                 = "copy-original-realm-setup"
)

// DeployKeycloak deploys a keycloak instance via the codecentric Helm Chart.
func DeployKeycloak(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNKeycloak{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Adding postgresql instance")
	err = addPostgreSQL(svc, comp)
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
		comp.GetName() + pgInstanceNameSuffix: {
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

	adminSecret, err := common.AddCredentialsSecret(comp, svc, []string{adminPWSecretField})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot generate admin secret: %s", err))
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed connection details for keycloak admin: %s", err))
	}

	svc.Log.Info("Adding Network policy for keycloak")

	sourceNS := []string{
		comp.GetClaimNamespace(),
	}
	err = common.CreateNetworkPolicy(sourceNS, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create net pol: %w", err))
	}

	svc.SetConnectionDetail(adminPWConnectionDetailsField, cd[adminPWSecretField])
	svc.SetConnectionDetail(adminConnectionDetailsField, []byte("admin"))
	svc.SetConnectionDetail(hostConnectionDetailsField, []byte(fmt.Sprintf("%s-%s.%s.svc.cluster.local", comp.GetName(), serviceSuffix, comp.GetInstanceNamespace())))

	err = addRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create release: %s", err))
	}

	return nil
}

func addPostgreSQL(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) error {
	// Unfortunately k8up and stackgres backups don't match up very well...
	// if no daily backup is set we just do the default.
	retention := 6
	if comp.Spec.Parameters.Backup.Retention.KeepDaily != 0 {
		retention = comp.Spec.Parameters.Backup.Retention.KeepDaily
	}

	configs := map[string]string{
		"ignore_startup_parameters": "extra_float_digits, search_path",
	}

	configBytes, err := json.Marshal(configs)
	if err != nil {
		return err
	}

	params := &vshnv1.VSHNPostgreSQLParameters{
		Size:      comp.Spec.Parameters.Size,
		Instances: 1,
		Backup: vshnv1.VSHNPostgreSQLBackup{
			Retention:          retention,
			DeletionProtection: true,
		},
		Service: vshnv1.VSHNPostgreSQLServiceSpec{
			PgBouncerSettings: &sgv1.SGPoolingConfigSpecPgBouncerPgbouncerIni{
				Pgbouncer: k8sruntime.RawExtension{
					Raw: configBytes,
				},
			},
		},
	}

	if comp.Spec.Parameters.Service.PostgreSQLParameters != nil {
		err := mergo.Merge(params, comp.Spec.Parameters.Service.PostgreSQLParameters, mergo.WithOverride)
		if err != nil {
			return err
		}

		// Mergo currently has a bug with merging bools: https://github.com/darccio/mergo/issues/249
		// It's not possible to override true with false, so it won't merge this if the users disables it.
		if !comp.Spec.Parameters.Service.PostgreSQLParameters.Backup.DeletionProtection {
			params.Backup.DeletionProtection = false
		}
	}

	pg := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + pgInstanceNameSuffix,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: *params,
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      pgSecretName,
					Namespace: comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err = common.CreateNetworkPolicy([]string{comp.GetInstanceNamespace()}, pg.GetInstanceNamespace(), pg.GetName()+"-keycloak", svc)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(pg)
}

func addRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) error {
	release, err := newRelease(ctx, svc, comp, adminSecret)
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResource(release)
}

func getResources(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) (common.Resources, error) {
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return common.Resources{}, err
	}

	res := common.GetResources(&comp.Spec.Parameters.Size, resources)

	return res, nil
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (map[string]any, error) {
	values := map[string]any{}

	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + pgInstanceNameSuffix)
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
			"value": "admin",
		},
		{
			"name": "KEYCLOAK_ADMIN_PASSWORD",
			"valueFrom": map[string]any{
				"secretKeyRef": map[string]any{
					"name": adminSecret,
					"key":  adminPWSecretField,
				},
			},
		},
		{
			"name":  "KC_DB_URL_PROPERTIES",
			"value": "?sslmode=verify-full&sslrootcert=/certs/ca.crt",
		},
		{
			"name": "JAVA_OPTS_APPEND",
			"value": `-Djava.awt.headless=true
-Djgroups.dns.query={{ include "keycloak.fullname" . }}-headless`,
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
			"name":     "custom-setup",
			"emptyDir": nil,
		},
		{
			"name": "keycloak-dist",
		},
		{
			"name": "postgresql-certs",
			"secret": map[string]any{
				"secretName":  pgSecretName,
				"defaultMode": 420,
			},
		},
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
			"name":      "custom-setup",
			"mountPath": "/opt/keycloak/setup",
		},
		{
			"name":      "postgresql-certs",
			"mountPath": "/certs/",
		},
	}

	extraVolumeMounts, err := toYAML(extraVolumeMountsMap)
	if err != nil {
		return nil, err
	}

	values = map[string]any{
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
		"serviceAccount": map[string]any{
			"automountServiceAccountToken": "false",
			"imagePullSecrets": []map[string]any{
				{
					"name": pullsecretName,
				},
			},
		},
		"resources": map[string]any{
			"requests": map[string]any{
				"memory": res.ReqMem,
				"cpu":    res.ReqCPU,
			},
			"limits": map[string]any{
				"memory": res.Mem,
				"cpu":    res.CPU,
			},
		},
		"nodeSelector": nodeSelector,
		"dbchecker": map[string]any{
			"enabled": true,
		},
		// See https://github.com/keycloak/keycloak/issues/11286
		// readOnlyRootFilesystem: true
		"securityContext": map[string]any{
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
	}

	fqdn := comp.Spec.Parameters.Service.FQDN
	if fqdn != "" {
		values["ingress"] = map[string]any{
			"hostname": fqdn,
		}
	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, adminSecret string) (*xhelmv1.Release, error) {
	values, err := newValues(ctx, svc, comp, adminSecret)
	if err != nil {
		return nil, err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName())
	if err != nil {
		return nil, fmt.Errorf("cannot get observed release values: %w", err)
	}

	observedVersion, err := maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return nil, fmt.Errorf("cannot set keycloak version for release: %w", err)
	}

	err = addInitContainer(values, observedVersion)
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

func addInitContainer(values map[string]any, version string) error {
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

	extraInitContainers, err := toYAML(extraInitContainersMap)
	if err != nil {
		return err
	}

	values["extraInitContainers"] = extraInitContainers

	return nil
}
