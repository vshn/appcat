package vshnkeycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AddIngress adds an inrgess to the Keycloak instance.
func AddIngress(_ context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	fqdn := comp.Spec.Parameters.Service.FQDN
	if fqdn == "" {
		return nil
	}

	if common.IsHTTPRouteMode(svc) {
		return addKeycloakHTTPRoute(comp, svc, fqdn)
	}

	values, err := common.GetDesiredReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get desired release values: %s", err))
	}

	svc.Log.Info("Adding ingress")
	ingress, err := buildKeycloakIngress(comp, svc, fqdn)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot generate ingress: %s", err))
	}

	err = common.CreateIngresses(comp, svc, []*netv1.Ingress{ingress}, runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create ingress: %s", err))
	}

	if svc.GetBoolFromCompositionConfig("isOpenshift") {
		err := addOpenShiftCa(svc, comp)
		if err != nil {
			svc.Log.Error(err, "cannot add openshift ca secret")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot add openshift ca secret: %s", err)))
		}
	}

	release := &xhelmv1.Release{}
	err = svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get desired release: %s", err))
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot marhal values: %s", err))
	}

	release.Spec.ForProvider.Values.Raw = vb

	err = svc.SetDesiredComposedResourceWithName(release, comp.GetName()+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set desired release: %s", err))
	}

	return nil
}

// buildKeycloakIngress generates an Ingress exposing only the Keycloak paths
// recommended for reverse proxy deployments. /admin/ is included only when
// DisableAdminAccess is false.
func buildKeycloakIngress(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime, fqdn string) (*netv1.Ingress, error) {
	base := strings.TrimSuffix(comp.Spec.Parameters.Service.RelativePath, "/")

	allowedPaths := []string{
		base + "/realms/",
		base + "/resources/",
		base + "/.well-known/",
	}
	if !comp.Spec.Parameters.Service.DisableAdminAccess {
		allowedPaths = append(allowedPaths, base+"/admin/", base+"/")
	}

	ingress, err := common.GenerateIngress(comp, svc, common.IngressConfig{
		FQDNs: []string{fqdn},
		ServiceConfig: common.IngressRuleConfig{
			RelPath:           allowedPaths[0],
			ServiceNameSuffix: "keycloakx-http",
			ServicePortName:   "https",
		},
		TlsCertBaseName: "keycloak",
	})
	if err != nil {
		return nil, err
	}

	pathType := netv1.PathTypePrefix
	backend := ingress.Spec.Rules[0].HTTP.Paths[0].Backend
	for _, p := range allowedPaths[1:] {
		ingress.Spec.Rules[0].HTTP.Paths = append(ingress.Spec.Rules[0].HTTP.Paths, netv1.HTTPIngressPath{
			Path:     p,
			PathType: &pathType,
			Backend:  backend,
		})
	}

	return ingress, nil
}

// addOpenShiftCa creates a separate secret just with the ca in it.
// This is required so that the ca on the route is properly set.
// For some reason openshift doesn't take the CA certificate from the ca.crt
// field of a tls secret... So we create a separate one which contains the CA
// cert in each mandatory field.
func addOpenShiftCa(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak) error {
	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + cdCertsSuffix)
	if err != nil {
		return err
	}

	keyName := "ca.crt"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "route-ca",
			Namespace: comp.GetInstanceNamespace(),
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			keyName:   cd[keyName],
			"tls.crt": cd[keyName],
			"tls.key": cd[keyName],
		},
	}

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"-route-ca", runtime.KubeOptionAllowDeletion)
}

func addKeycloakHTTPRoute(comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime, fqdn string) *xfnproto.Result {
	svc.Log.Info("Adding HTTPRoute for Keycloak")

	if res := common.ApplyHTTPRouteAsResult(comp, svc, common.HTTPRouteConfig{
		FQDNs: []string{fqdn},
		ServiceConfig: common.IngressRuleConfig{
			RelPath:           comp.Spec.Parameters.Service.RelativePath,
			ServiceNameSuffix: "keycloakx-http",
			ServicePortNumber: 8443,
		},
	}); res != nil {
		return res
	}

	// Keycloak serves TLS on port 8443 directly. kgateway defaults to plain
	// HTTP upstream, so attach a BackendConfigPolicy that originates TLS to
	// the keycloakx-http Service, validating against the CA secret created
	// by common.CreateTLSCerts ("tls-ca-certificate" in the instance ns).
	svcName := comp.GetName() + "-keycloakx-http"
	bcp := &unstructured.Unstructured{}
	bcp.SetAPIVersion("gateway.kgateway.dev/v1alpha1")
	bcp.SetKind("BackendConfigPolicy")
	bcp.SetName(comp.GetName() + "-backend-tls")
	bcp.SetNamespace(comp.GetInstanceNamespace())
	bcp.Object["spec"] = map[string]any{
		"targetRefs": []any{
			map[string]any{"group": "", "kind": "Service", "name": svcName},
		},
		"tls": map[string]any{
			"secretRef": map[string]any{"name": "tls-ca-certificate"},
		},
	}
	if err := svc.SetDesiredKubeObject(bcp, bcp.GetName()); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create BackendConfigPolicy: %s", err))
	}

	return nil
}
