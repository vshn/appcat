package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// serverCertSecretName matches the name hardcoded in common.CreateTLSCerts.
const serverCertSecretName = "tls-server-certificate"

func SetupTLSCertificates(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	_, err = common.CreateTLSCerts(ctx, comp.GetInstanceNamespace(), comp.GetName(), svc, nil)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot setup TLS for OpenBao: %w", err).Error())
	}
	return nil
}
