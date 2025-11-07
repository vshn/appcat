package vshnkeycloak

import (
	"context"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	values, err := common.GetDesiredReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get desired release values: %s", err))
	}

	svc.Log.Info("Adding ingress")
	ingress, err := common.GenerateIngress(comp, svc, common.IngressConfig{
		FQDNs: []string{fqdn},
		ServiceConfig: common.IngressRuleConfig{
			RelPath:           comp.Spec.Parameters.Service.RelativePath,
			ServiceNameSuffix: "keycloakx-http",
			ServicePortName:   "https",
		},
		TlsCertBaseName: "keycloak",
	})
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

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"-route-ca")
}
