package vshnkeycloak

import (
	"context"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AddIngress adds an inrgess to the Keycloak instance.
func AddIngress(_ context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if comp.Spec.Parameters.Service.FQDN == "" {
		return nil
	}

	values, err := common.GetDesiredReleaseValues(svc, comp.GetName()+"-release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get desired release values: %s", err))
	}

	svc.Log.Info("Enable ingress for release")
	enableIngresValues(svc, comp, values)

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

func enableIngresValues(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKeycloak, values map[string]any) {
	fqdn := comp.Spec.Parameters.Service.FQDN

	relPath := `'{{ tpl .Values.http.relativePath $ | trimSuffix " / " }}/'`
	if comp.Spec.Parameters.Service.RelativePath == "/" {
		relPath = "/"
	}

	tls := map[string]any{}
	if !common.IsSingleSubdomainOfRefDomain(fqdn, svc.Config.Data["ocpDefaultAppsDomain"]) {
		tls["hosts"] = []string{fqdn}
		tls["secretName"] = "keycloak-ingress-cert"
	}
	tlsConfig := []map[string]any{tls}

	values["ingress"] = map[string]any{
		"enabled":     true,
		"servicePort": "https",

		"rules": []map[string]any{
			{
				"host": fqdn,
				"paths": []map[string]any{
					{
						"path":     relPath,
						"pathType": "Prefix",
					},
				},
			},
		},
		"tls": tlsConfig,
	}

	if svc.Config.Data["ingress_annotations"] != "" {
		annotations := map[string]any{}

		err := yaml.Unmarshal([]byte(svc.Config.Data["ingress_annotations"]), annotations)
		if err != nil {
			svc.Log.Error(err, "cannot unmarshal ingress annotations from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal ingress annotations from input: %s", err)))
		}

		unstructured.SetNestedMap(values, annotations, "ingress", "annotations")
	}

	if svc.GetBoolFromCompositionConfig("isOpenshift") {
		err := addOpenShiftCa(svc, comp)
		if err != nil {
			svc.Log.Error(err, "cannot add openshift ca secret")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot add openshift ca secret: %s", err)))
		}
	}

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
