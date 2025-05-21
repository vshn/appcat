package vshnredis

import (
	"context"
	"fmt"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
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
)

func DeployRedis(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	// TODO Enable event forwarding 
	
	l.Info("Bootstrapping instance namespace and rbac rules")
	if err := common.BootstrapInstanceNs(ctx, comp, comp.GetServiceName(), comp.GetName()+"-instanceNs", svc); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	l.Info("Create credentials secret")
	secretName, err := common.AddCredentialsSecret(comp, svc, []string{passwordKey}, common.DisallowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create credentials secret: %w", err).Error())
	}

	l.Info("Creating tls certificate for redis instance")
	tlsOpts := &common.TLSOptions{
		AdditionalSans: []string{
			fmt.Sprintf("redis-headless.vshn-redis-%s.svc.cluster.local", comp.GetName()),
			fmt.Sprintf("redis-headless.vshn-redis-%s.svc", comp.GetName()),
		},
		CertOptions: []common.CertOptions{
			func(c *cmv1.Certificate) {
				c.Spec.CommonName = "vshn-appcat"
				c.Spec.Subject = &cmv1.X509Subject{Organizations: []string{"vshn-appcat-server"}}
			},
		},
	}

	if _, err := common.CreateTLSCerts(ctx, comp.GetInstanceNamespace(), comp.GetName(), svc, tlsOpts); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create TLS certificates: %w", err).Error())
	}

	l.Info("Creating helm release for redis instance")
	if err := createObjectHelmRelease(ctx, comp, svc, secretName); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create helm release: %w", err).Error())
	}

	l.Info("Get connection details from secret")
	if err := getConnectionDetails(comp, svc, secretName); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot build connection details: %w", err).Error())
	}

	return nil
}

// Create the helm release for the redis instance
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime, secretName string) error {
	values, err := newValues(ctx, svc, comp, secretName)
	if err != nil {
		return err
	}

	observedValues, err := common.GetObservedReleaseValues(svc, comp.GetName()+"-release")
	if err == nil {
		_, err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
		if err != nil {
			return fmt.Errorf("cannot set redis version for release: %w", err)
		}
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
		return nil, fmt.Errorf("cannot fetch nodeSelector: %w", err)
	}

	values := map[string]any{
		"fullnameOverride": comp.GetName(),
		"architecture":     "standalone",
		"global": map[string]any{
			"security": map[string]any{
				"allowInsecureImages": true,
			},
		},
		"image": map[string]any{
			"repository": "bitnami/redis",
			"tag":        comp.Spec.Parameters.Service.Version,
		},
		"auth": map[string]any{
			"enabled":                   true,
			"existingSecret":            secret,
			"existingSecretPasswordKey": passwordKey,
		},
		"tls": map[string]any{
			"enabled":         true,
			"authClients":     true,
			"autoGenerated":   false,
			"existingSecret":  serverCertificateSecretName,
			"certFilename":    "tls.crt",
			"certKeyFilename": "tls.key",
			"certCAFilename":  "ca.crt",
		},
		"metrics": map[string]any{
			"enabled": true,
			"extraEnvVars": []map[string]string{
				{"name": "REDIS_EXPORTER_SKIP_TLS_VERIFICATION", "value": "true"},
				{"name": "REDIS_EXPORTER_INCL_SYSTEM_METRICS", "value": "true"},
			},
			"serviceMonitor": map[string]any{
				"enabled":   true,
				"namespace": comp.GetInstanceNamespace(),
			},
			"containerSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"resources": common.GetBitnamiNano(),
		},
		"master": map[string]any{
			"persistence": map[string]any{"size": comp.Spec.Parameters.Size.Disk},
			"podSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
			"containerSecurityContext": map[string]any{
				"enabled": !svc.GetBoolFromCompositionConfig("isOpenshift"),
			},
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
			"nodeSelector": nodeSelector,
		},
		"commonConfiguration": map[string]any{},
	}

	if registry := svc.Config.Data["imageRegistry"]; registry != "" {
		err := common.SetNestedObjectValue(values, []string{"global", "imageRegistry"}, registry)
		if err != nil {
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
				Name:       comp.Name,
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
				FieldPath:  "data[ca.crt]"
			},
			ToConnectionSecretKey:  "ca.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "tls-client-certificate",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "data[tls.crt]"
			},
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
	rel, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-release", cd...)
	if err != nil {
		return nil, err
	}

	rel.Spec.ForProvider.Chart.Name = "redis"
	return rel, nil
}

func getConnectionDetails(comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime, secret string) error {
	pw, err := svc.GetObservedComposedResourceConnectionDetails(secret)
	if err != nil {
		return fmt.Errorf("cannot read redis password: %w", err)
	}
	pass := string(pw[passwordKey])

	host := fmt.Sprintf("%s.%s.svc.cluster.local", comp.GetName(), comp.GetInstanceNamespace())
	url := fmt.Sprintf("redis://%s:%s@%s:%s", redisUser, pass, host, redisPort)

	svc.SetConnectionDetail(redisHostConnectionDetailsField, []byte(host))
	svc.SetConnectionDetail(redisPortConnectionDetailsField, []byte(redisPort))
	svc.SetConnectionDetail(redisUsernameConnectionDetailsField, []byte(redisUser))
	svc.SetConnectionDetail(redisPasswordConnectionDetailsField, []byte(pass))
	svc.SetConnectionDetail(redisURLConnectionDetailsField, []byte(url))
	return nil
}
