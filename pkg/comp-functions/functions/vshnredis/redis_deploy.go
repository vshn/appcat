package vshnredis

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	redisPort                           = "6379"
	redisUser                           = "default"
	passwordKey                         = "root-password"
	serverCertificateSecretName         = "tls-server-certificate"
	redisHostConnectionDetailsField     = "REDIS_HOST"
	redisPortConnectionDetailsField     = "REDIS_PORT"
	redisUsernameConnectionDetailsField = "REDIS_USERNAME"
	redisPasswordConnectionDetailsField = "REDIS_PASSWORD"
	redisURLConnectionDetailsField      = "REDIS_URL"
	sentinelHostsConnectionDetailsField = "SENTINEL_HOSTS"

	redisRelease = "release"
)

//go:embed script/scaling.sh
var redisScalingScript string

// DeployRedis will deploy the objects to provision redis instance
func DeployRedis(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if err := common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), "namespace-conditions", svc); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	secretName, err := common.AddCredentialsSecret(comp, svc, []string{passwordKey}, common.DisallowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create credentials secret: %w", err).Error())
	}

	additionalSans := []string{
		fmt.Sprintf("redis-headless.vshn-redis-%s.svc.cluster.local", comp.GetName()),
		fmt.Sprintf("redis-headless.vshn-redis-%s.svc", comp.GetName()),
	}
	if comp.GetInstances() > 1 {
		additionalSans = append(additionalSans, fmt.Sprintf("redis-master.vshn-redis-%s.svc.cluster.local", comp.GetName()))
		additionalSans = append(additionalSans, fmt.Sprintf("redis-master.vshn-redis-%s.svc", comp.GetName()))
		for i := range comp.GetInstances() {
			additionalSans = append(additionalSans, fmt.Sprintf("redis-node-%d.redis-headless.vshn-redis-%s.svc.cluster.local", i, comp.GetName()))
			additionalSans = append(additionalSans, fmt.Sprintf("redis-node-%d.redis-headless.vshn-redis-%s.svc", i, comp.GetName()))
		}
	}

	tlsOpts := &common.TLSOptions{
		AdditionalSans: additionalSans,
	}

	_, _, err = createMTLSCerts(comp.GetInstanceNamespace(), comp.GetName(), svc, tlsOpts)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create mTLS certificates: %w", err).Error())
	}

	if err := createObjectHelmRelease(ctx, comp, svc, secretName); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create helm release: %w", err).Error())
	}

	if err := getConnectionDetails(comp, svc, secretName); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot set connection details: %w", err).Error())
	}

	return nil
}

func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime, secretName string) error {
	values, err := newValues(ctx, svc, comp, secretName)
	if err != nil {
		return err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, redisRelease)
	if err != nil {
		return fmt.Errorf("cannot get observed release values: %w", err)
	}

	_, err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return fmt.Errorf("cannot set redis version for release: %w", err)
	}

	rel, err := newRelease(ctx, svc, values, comp)
	if err != nil {
		return err
	}

	err = migrateRedis(ctx, svc, comp)
	if err != nil {
		svc.Log.Error(err, "cannot create migration job")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot create migration job: %s", err)))
	}

	if err := svc.SetDesiredComposedResourceWithName(rel, redisRelease); err != nil {
		return err
	}

	return svc.AddObservedConnectionDetails(redisRelease)
}

func newValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis, secret string) (map[string]any, error) {
	l := svc.Log

	plan := comp.Spec.Parameters.Size.Plan
	if plan == "" {
		plan = svc.Config.Data["defaultPlan"]
	}

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch plans from config: %w", err)
	}

	vshnSize := comp.GetSize()
	res, errs := common.GetResources(&vshnSize, resources)
	if len(errs) != 0 {
		l.Error(errs[0], "resource calculation error")
	}

	nodeSelector, err := utils.FetchNodeSelectorFromConfig(ctx, svc, plan, comp.Spec.Parameters.Scheduling.NodeSelector)
	if err != nil {
		return nil, err
	}

	commonConfig := ""
	if comp.Spec.Parameters.Service.RedisSettings != "" {
		commonConfig = comp.Spec.Parameters.Service.RedisSettings
	}

	automountServiceAccountToken := false
	masterService := false
	rbacCreate := false
	if comp.GetInstances() > 1 {
		automountServiceAccountToken = true
		masterService = true
		rbacCreate = true
	}

	sidecars, err := utils.FetchSidecarsFromConfig(ctx, svc)

	if err != nil {
		err = fmt.Errorf("cannot get sideCars from config: %w", err)
		return nil, err
	}

	tagMap := map[string]string{
		"7": "7.2.12",
		"6": "6.2.21",
		"8": "8.0.5",
	}

	redisMajorVersion := string(comp.Spec.Parameters.Service.Version[0])

	values := map[string]any{
		"fullnameOverride": "redis",
		"architecture":     "replication",
		"sentinel": map[string]any{
			"enabled": true,
			"resources": map[string]any{
				"limits": map[string]any{
					"ephemeral-storage": "50Mi",
				},
			},
			"masterService": map[string]any{
				"enabled": masterService,
			},
		},
		"replica": map[string]any{
			"replicaCount":                 comp.GetInstances(),
			"automountServiceAccountToken": automountServiceAccountToken,
			"persistence": map[string]any{
				"size": res.Disk,
			},
			"podSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"containerSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
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
		},
		"kubectl": map[string]any{
			"podSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"containerSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
		},
		"global": map[string]any{
			"security": map[string]any{
				"allowInsecureImages": true,
			},
		},
		"rbac": map[string]any{
			"create": rbacCreate,
		},
		"image": map[string]any{
			"tag": comp.Spec.Parameters.Service.Version,
		},

		"auth": map[string]any{
			"enabled":                   true,
			"existingSecret":            secret,
			"existingSecretPasswordKey": passwordKey,
		},

		"tls": map[string]any{
			"enabled":         comp.Spec.Parameters.TLS.TLSEnabled,
			"authClients":     comp.Spec.Parameters.TLS.TLSAuthClients,
			"autoGenerated":   false,
			"existingSecret":  serverCertificateSecretName,
			"certFilename":    "tls.crt",
			"certKeyFilename": "tls.key",
			"certCAFilename":  "ca.crt",
		},

		"metrics": map[string]any{
			"enabled": true,

			"extraEnvVars": []map[string]string{
				{
					"name":  "REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
					"value": "true",
				},
				{
					"name":  "REDIS_EXPORTER_INCL_SYSTEM_METRICS",
					"value": "true",
				},
			},

			"serviceMonitor": map[string]any{
				"enabled":   true,
				"namespace": comp.GetInstanceNamespace(),
			},

			"containerSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
		},
		"commonConfiguration": commonConfig,

		"networkPolicy": map[string]any{
			"enabled": false,
		},
	}

	common.MergeSidecarsIntoValues(values, sidecars)

	if registry := svc.Config.Data["imageRegistry"]; registry != "" {
		_ = common.SetNestedObjectValue(values, []string{"global", "imageRegistry"}, registry)
	}

	if imageRepositoryPrefix := svc.Config.Data["imageRepositoryPrefix"]; imageRepositoryPrefix != "" {
		if err := common.SetNestedObjectValue(values, []string{"image"}, map[string]any{
			"repository": fmt.Sprintf("%s/redis", imageRepositoryPrefix),
			"tag":        tagMap[redisMajorVersion],
		}); err != nil {
			return nil, err
		}

		if err := common.SetNestedObjectValue(values, []string{"sentinel", "image"}, map[string]any{
			"repository": fmt.Sprintf("%s/redis-sentinel", imageRepositoryPrefix),
			"tag":        tagMap[redisMajorVersion],
		}); err != nil {
			return nil, err
		}

		if err := common.SetNestedObjectValue(values, []string{"metrics", "image"}, map[string]any{
			"repository": fmt.Sprintf("%s/redis-exporter", imageRepositoryPrefix),
			"tag":        "1.76.0-debian-12-r0",
		}); err != nil {
			return nil, err
		}

		if err := common.SetNestedObjectValue(values, []string{"kubectl", "image"}, map[string]any{
			"repository": fmt.Sprintf("%s/kubectl", imageRepositoryPrefix),
		}); err != nil {
			return nil, err
		}

	}

	return values, nil
}

func newRelease(ctx context.Context, svc *runtime.ServiceRuntime, values map[string]any, comp *vshnv1.VSHNRedis) (*xhelmv1.Release, error) {
	cd := []xhelmv1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       comp.GetName() + "-credentials-secret",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data." + passwordKey,
			},
			ToConnectionSecretKey:  "REDIS_PASSWORD",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-client-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[ca.crt]"},
			ToConnectionSecretKey:  "ca.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-client-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[tls.crt]"},
			ToConnectionSecretKey:  "tls.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-client-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[tls.key]"},
			ToConnectionSecretKey:  "tls.key",
			SkipPartOfReleaseCheck: true,
		},
	}

	rel, err := common.NewRelease(ctx, svc, comp, values, redisRelease, cd...)
	if err != nil {
		return nil, err
	}

	return rel, nil
}

func getConnectionDetails(comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime, secretName string) error {
	redisPassword, err := getRedisRootPassword(secretName, svc)
	if err != nil {
		return err
	}

	host := fmt.Sprintf("redis-headless.vshn-redis-%s.svc.cluster.local", comp.GetName())
	if comp.GetInstances() > 1 {
		host = fmt.Sprintf("redis-master.vshn-redis-%s.svc.cluster.local", comp.GetName())

		sentinel_hosts := []string{}
		for i := 0; i < comp.GetInstances(); i++ {
			sentinel_hosts = append(sentinel_hosts, fmt.Sprintf("redis-node-%d.redis-headless.vshn-redis-%s.svc.cluster.local", i, comp.GetName()))
		}
		svc.SetConnectionDetail(sentinelHostsConnectionDetailsField, []byte(strings.Join(sentinel_hosts, ",")))
	}
	url := fmt.Sprintf("rediss://%s:%s@%s:%s", redisUser, string(redisPassword), host, redisPort)

	svc.SetConnectionDetail(redisHostConnectionDetailsField, []byte(host))
	svc.SetConnectionDetail(redisPortConnectionDetailsField, []byte(redisPort))
	svc.SetConnectionDetail(redisUsernameConnectionDetailsField, []byte(redisUser))
	svc.SetConnectionDetail(redisPasswordConnectionDetailsField, redisPassword)
	svc.SetConnectionDetail(redisURLConnectionDetailsField, []byte(url))

	return nil
}

func getRedisRootPassword(secretName string, svc *runtime.ServiceRuntime) ([]byte, error) {
	secret := &corev1.Secret{}

	err := svc.GetObservedKubeObject(secret, secretName)

	if err != nil {
		if err == runtime.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return secret.Data[passwordKey], nil
}

func migrateRedis(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis) error {

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets"},
			Verbs:     []string{"get", "watch", "list"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets/scale"},
			Verbs:     []string{"patch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"get", "create", "watch", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "list"},
		},
	}

	err := common.AddSaWithRole(ctx, svc, rules, comp.GetName(), comp.GetInstanceNamespace(), "pvc-migration", true)
	if err != nil {
		return fmt.Errorf("failed to add PVC-Migration SA with Role: %w", err)
	}

	migrationJobDesired := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-migrationjob",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: "OnFailure",
					Containers: []corev1.Container{
						{
							Name:    "migrationjob",
							Image:   svc.Config.Data["kubectl_image"],
							Command: []string{"sh", "-c"},
							Args:    []string{redisScalingScript},
							Env: []corev1.EnvVar{
								{
									Name:  "INSTANCE_NAMESPACE",
									Value: comp.GetInstanceNamespace(),
								},
								{
									Name:  "INSTANCE_NAME",
									Value: comp.GetName(),
								},
							},
						},
					},
					ServiceAccountName: "sa-pvc-migration",
				},
			},
		},
	}

	err = svc.SetDesiredKubeObject(migrationJobDesired, comp.GetName()+"-scalingjob")
	if err != nil {
		err = fmt.Errorf("cannot create scalingJob: %w", err)
		return err
	}

	return nil
}
