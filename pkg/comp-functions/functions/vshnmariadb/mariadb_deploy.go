package vshnmariadb

import (
	"context"
	"encoding/json"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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
		err = fmt.Errorf("cannot get observed composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "mariadb", comp.GetName()+"-instanceNs", svc)
	if err != nil {
		err = fmt.Errorf("cannot bootstrap instance namespace: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating tls certificate for mariadb instance")
	err = common.CreateTlsCerts(ctx, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		err = fmt.Errorf("cannot create tls certificate: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating helm release for mariadb instance")
	err = createObjectHelmRelease(ctx, comp, svc)
	if err != nil {
		err = fmt.Errorf("cannot create helm release: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Creating network policies mariadb instance")
	sourceNs := []string{comp.GetClaimNamespace()}
	if svc.GetBoolFromCompositionConfig("slosEnabled") {
		sourceNs = append(sourceNs, svc.Config.Data["slosNs"])
	}
	err = common.CreateNetworkPolicy(ctx, sourceNs, comp.GetInstanceNamespace(), comp.GetName(), svc)
	if err != nil {
		err = fmt.Errorf("cannot create helm release: %w", err)
		return runtime.NewFatalResult(err)
	}

	l.Info("Get connection details from secret")
	getConnectionDetails(comp, svc)
	return nil
}

// Create the helm release for the mariadb instance
func createObjectHelmRelease(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) error {

	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])

	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		err = fmt.Errorf("cannot fetch plans from the composition config, maybe they are not set: %w", err)
		return err
	}

	reqMem, reqCPU, mem, cpu, disk := common.GetResources(&comp.Spec.Parameters.Size, resources)

	values := map[string]interface{}{
		"fullnameOverride": comp.GetName(),
		"replicaCount":     1,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"memory": reqMem,
				"cpu":    reqCPU,
			},
			"limits": map[string]interface{}{
				"memory": mem,
				"cpu":    cpu,
			},
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
			"size":         disk,
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
	}

	vb, err := json.Marshal(values)
	if err != nil {
		err = fmt.Errorf("cannot marshal helm values: %w", err)
		return err
	}

	r := &xhelmbeta1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: xhelmbeta1.ReleaseSpec{
			ForProvider: xhelmbeta1.ReleaseParameters{
				Chart: xhelmbeta1.ChartSpec{
					Repository: svc.Config.Data["chartRepository"],
					Version:    svc.Config.Data["chartVersion"],
					Name:       "mariadb-galera",
				},
				Namespace: comp.GetInstanceNamespace(),
				ValuesSpec: xhelmbeta1.ValuesSpec{
					Values: k8sruntime.RawExtension{
						Raw: vb,
					},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "helm",
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      comp.GetName() + "-connection",
					Namespace: comp.GetInstanceNamespace(),
				},
			},
			ConnectionDetails: []xhelmbeta1.ConnectionDetail{
				{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       comp.GetName(),
						Namespace:  comp.GetInstanceNamespace(),
						FieldPath:  "data.mariadb-root-password",
					},
					ToConnectionSecretKey: "MARIADB_PASSWORD",
				},
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
			},
		},
	}

	err = svc.AddObservedConnectionDetails(comp.Name + "-release")
	if err != nil {
		return err
	}

	return svc.SetDesiredComposedResourceWithName(r, comp.Name+"-release")
}

func getConnectionDetails(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) {
	mariadbHost := comp.GetName() + ".vshn-mariadb-" + comp.GetName() + ".svc.cluster.local"
	mariadbURL := fmt.Sprintf("mysql://%s:%s", mariadbHost, mariadbPort)

	svc.SetConnectionDetail("MARIADB_HOST", []byte(mariadbHost))
	svc.SetConnectionDetail("MARIADB_PORT", []byte(mariadbPort))
	svc.SetConnectionDetail("MARIADB_USERNAME", []byte(mariadbUser))
	svc.SetConnectionDetail("MARIADB_URL", []byte(mariadbURL))
}
