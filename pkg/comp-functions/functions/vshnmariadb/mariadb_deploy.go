package vshnmariadb

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

const (
	mariadbPort = "3306"
	mariadbUser = "root"
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
		"mariadbConfiguration": comp.Spec.Parameters.Service.MariadbSettings,
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
	}
	rel, err := common.NewRelease(ctx, svc, comp, values, cd...)
	rel.Spec.ForProvider.Chart.Name = comp.GetServiceName() + "-galera"

	return rel, err
}
