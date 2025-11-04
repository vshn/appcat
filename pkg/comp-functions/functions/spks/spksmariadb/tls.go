package spksmariadb

import (
	"context"
	"fmt"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelm "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	spksv1alpha1 "github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleTLS will enable TLS if it's specified in the composite.
// It will then deploy certmanager objects to generate certificates.
func HandleTLS(ctx context.Context, comp *spksv1alpha1.CompositeMariaDBInstance, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite for spks redis: %w", err))
	}

	if !comp.Spec.Parameters.TLS {
		svc.SetConnectionDetail("ca.crt", []byte("N/A"))
		return runtime.NewNormalResult("TLS not enabled")
	}

	err = enableTLS(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot enable TLS: %s", err))
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails("galera-cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get connection details of haproxy: %s", err.Error()))
	}

	tlsOpts := &common.TLSOptions{
		AdditionalSans: []string{
			string(cd["endpoint"]),
			// from https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/tls.md#mariadb-certificate-specification
			"127.0.0.1",
			"localhost",
			fmt.Sprintf("mariadb.%s.svc.cluster.local", comp.GetName()),
			fmt.Sprintf("mariadb.%s.svc", comp.GetName()),
			fmt.Sprintf("mariadb.%s", comp.GetName()),
			"mariadb",
			fmt.Sprintf("*.mariadb-internal.%s.svc.cluster.local", comp.GetName()),
			fmt.Sprintf("*.mariadb-internal.%s.svc", comp.GetName()),
			fmt.Sprintf("*.mariadb-internal.%s", comp.GetName()),
			"*.mariadb-internal",
			fmt.Sprintf("mariadb-primary.%s.svc.cluster.local", comp.GetName()),
			fmt.Sprintf("mariadb-primary.%s.svc", comp.GetName()),
			fmt.Sprintf("mariadb-primary.%s", comp.GetName()),
			"mariadb-primary",
			fmt.Sprintf("mariadb-secondary.%s.svc.cluster.local", comp.GetName()),
			fmt.Sprintf("mariadb-secondary.%s.svc", comp.GetName()),
			fmt.Sprintf("mariadb-secondary.%s", comp.GetName()),
			"mariadb-secondary",
		},

		AdditionalOutputFormats: []v1.CertificateAdditionalOutputFormat{
			{
				Type: v1.CertificateOutputFormatCombinedPEM,
			},
		},

		KubeOptions: []runtime.KubeObjectOption{
			runtime.KubeOptionAddLabels(map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			}),
		},
	}

	_, err = common.CreateTLSCerts(ctx, comp.GetName(), comp.GetName(), svc, tlsOpts)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot apply certificates: %s", err.Error()))
	}

	svc.AddObservedConnectionDetails(comp.GetName() + "-server-cert")

	return nil
}

func enableTLS(svc *runtime.ServiceRuntime, comp *spksv1alpha1.CompositeMariaDBInstance) error {
	galeraCluster := &xhelm.Release{}

	err := svc.GetDesiredComposedResourceByName(galeraCluster, "galera-cluster")
	if err != nil {
		return fmt.Errorf("cannot get desired galera cluster relase")
	}

	values, err := common.GetReleaseValues(galeraCluster)
	if err != nil {
		return fmt.Errorf("cannot get values for galera cluster: %w", err)
	}

	err = unstructured.SetNestedField(values, comp.Spec.Parameters.TLS, "tls", "enabled")
	if err != nil {
		return fmt.Errorf("cannot enable TLS: %s", err)
	}

	err = unstructured.SetNestedField(values, comp.Spec.Parameters.RequireTLS, "tls", "required")
	if err != nil {
		return fmt.Errorf("cannot enable TLS: %s", err)
	}

	err = common.SetReleaseValues(galeraCluster, values)
	if err != nil {
		return fmt.Errorf("cannot set the galera values for TLS: %w", err)
	}

	err = svc.SetDesiredComposedResourceWithName(galeraCluster, "galera-cluster")
	if err != nil {
		return fmt.Errorf("cannot set galera cluster: %w", err)
	}

	return nil
}
