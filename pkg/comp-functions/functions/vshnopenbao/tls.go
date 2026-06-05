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

	name := comp.GetName()
	ns := comp.GetInstanceNamespace()

	opts := &common.TLSOptions{
		// Wildcard covers every StatefulSet pod addressed via the headless service:
		// {name}-N.{name}-internal.{ns}.svc.cluster.local
		AdditionalSans: []string{
			fmt.Sprintf("*.%s-internal.%s.svc.cluster.local", name, ns),
			fmt.Sprintf("%s-internal.%s.svc.cluster.local", name, ns),
		},
	}

	_, err = common.CreateTLSCerts(ctx, ns, name, svc, opts)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot setup TLS for OpenBao: %w", err).Error())
	}
	return nil
}
