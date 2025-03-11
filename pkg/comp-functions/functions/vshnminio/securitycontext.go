package vshnminio

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetSecurityContext patches the Minio statefulset to contain the correct securityContext.
// Unfortunately the helm chart does not expose this value.
func SetSecurityContext(_ context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		err = fmt.Errorf("cannot get observed composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	if comp.Spec.Parameters.Service.Mode == "distributed" {
		obj := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.GetName(),
				Namespace: comp.GetInstanceNamespace(),
			},
		}

		err = common.SetSELinuxSecurityContextStatefulset(obj, comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot set SELinux security context: %s", err))
		}
	} else {
		obj := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      comp.GetName(),
				Namespace: comp.GetInstanceNamespace(),
			},
		}

		err = common.SetSELinuxSecurityContextDeployment(obj, comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot set SELinux security context: %s", err))
		}
	}

	return nil
}
