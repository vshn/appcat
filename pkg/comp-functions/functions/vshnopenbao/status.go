package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// UpdateStatus is the single step responsible for writing all status fields back
// to the composite. No other step should call SetDesiredCompositeStatus.
func UpdateStatus(_ context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	initSecret := &corev1.Secret{}
	observerName := comp.GetName() + initOutputSecretSuffix + observerSuffix
	if err := svc.GetObservedKubeObject(initSecret, observerName); err == nil && len(initSecret.Data) > 0 {
		comp.SetInitialized(true)
	}

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot set composite status: %w", err).Error())
	}
	return nil
}
