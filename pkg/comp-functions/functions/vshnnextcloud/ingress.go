package vshnnextcloud

import (
	"context"
	"errors"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
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

	ingressConfig := common.IngressConfig{
		FQDNs: comp.Spec.Parameters.Service.FQDN,
		ServiceConfig: common.IngressRuleConfig{
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
