package vshnnextcloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// AddIngress adds an inrgess to the Nextcloud instance.
func AddIngress(_ context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if len(comp.Spec.Parameters.Service.FQDN) == 0 {
		return runtime.NewFatalResult(fmt.Errorf("FQDN array is empty, but requires at least one entry, %w", errors.New("empty fqdn")))
	}

	if svc.Config.Data["routeType"] == "HTTPRoute" {
		return addNextcloudHTTPRoute(comp, svc)
	}

	var svcNameSuffix string
	if !strings.Contains(comp.GetName(), "nextcloud") {
		svcNameSuffix = "nextcloud"
	}

	ingressConfig := common.IngressConfig{
		FQDNs: comp.Spec.Parameters.Service.FQDN,
		ServiceConfig: common.IngressRuleConfig{
			ServiceNameSuffix: svcNameSuffix,
			ServicePortNumber: 8080,
		},
		TlsCertBaseName: "nextcloud",
	}

	ingresses, err := common.GenerateBundledIngresses(comp, svc, ingressConfig)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Could not generate ingresses: %w", err))
	}

	common.CreateIngresses(comp, svc, ingresses)

	return nil
}

func addNextcloudHTTPRoute(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {
	gatewayName := svc.Config.Data["httpGatewayName"]
	gatewayNamespace := svc.Config.Data["httpGatewayNamespace"]

	var svcNameSuffix string
	if !strings.Contains(comp.GetName(), "nextcloud") {
		svcNameSuffix = "nextcloud"
	}

	svc.Log.Info("Adding HTTPRoute for Nextcloud")

	route, err := common.GenerateHTTPRoute(comp, svc, common.HTTPRouteConfig{
		FQDNs: comp.Spec.Parameters.Service.FQDN,
		ServiceConfig: common.IngressRuleConfig{
			ServiceNameSuffix: svcNameSuffix,
			ServicePortNumber: 8080,
		},
		GatewayName:      gatewayName,
		GatewayNamespace: gatewayNamespace,
	})
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot generate HTTPRoute: %w", err))
	}

	err = common.CreateHTTPRoutes(svc, []*gatewayv1.HTTPRoute{route})
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create HTTPRoute: %w", err))
	}

	serviceName := comp.GetName()
	if svcNameSuffix != "" {
		serviceName = serviceName + "-" + svcNameSuffix
	}

	grant, err := common.GenerateReferenceGrant(comp, svc, gatewayNamespace, serviceName)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot generate ReferenceGrant: %w", err))
	}

	err = common.CreateReferenceGrants(svc, []*gatewayv1beta1.ReferenceGrant{grant})
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create ReferenceGrant: %w", err))
	}

	return nil
}
