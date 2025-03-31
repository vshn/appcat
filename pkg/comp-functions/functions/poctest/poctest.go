package poctest

import (
	"context"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type forgejoState struct {
	desired   forgejoDesired
	observed  forgejoObserved
	composite *vshnv1.VSHNForgejo
}

type forgejoDesired struct {
	Namespace *corev1.Namespace
	Configmap *corev1.ConfigMap
}

type forgejoObserved struct {
}

func (f *forgejoState) GetDesiredState() any {
	return f.desired
}

func test(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {

	svc.GetObservedComposite(comp)

	state := &forgejoState{}

	state.desired.Namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
	}

	state.desired.Configmap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetName(),
		},
		Data: map[string]string{
			"test": "test",
		},
	}

	svc.ApplyState(state)

	return nil
}
