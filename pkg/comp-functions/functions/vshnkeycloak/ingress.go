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
)

// AddIngress adds an inrgess to the Keycloak instance.
func AddIngress(_ context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNKeycloak{}

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
	enableIngresValues(comp, values)

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

func enableIngresValues(comp *vshnv1.VSHNKeycloak, values map[string]any) {
	fqdn := comp.Spec.Parameters.Service.FQDN

	if fqdn != "" {
		relPath := `'{{ tpl .Values.http.relativePath $ | trimSuffix " / " }}/'`
		if comp.Spec.Parameters.Service.RelativePath == "/" {
			relPath = "/"
		}

		values["ingress"] = map[string]any{
			"enabled":     true,
			"servicePort": "https",
			"annotations": map[string]string{
				// This forces tls between nginx and keycloak
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
			},
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
			"tls": []map[string]any{
				{
					"hosts": []string{
						fqdn,
					},
				},
			},
		}
	}
}
