package poctest

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type forgejoState struct {
	desired   forgejoObjects
	observed  forgejoObjects
	composite *vshnv1.VSHNForgejo
	svc       *runtime.ServiceRuntime
}

type forgejoObjects struct {
	Namespace *corev1.Namespace
	Configmap *corev1.ConfigMap
	// Release   *xhelm.Release
	// Dynamic []client.Object
}

func (f *forgejoState) GetDesiredState() any {
	return f.desired
}

func (f *forgejoState) SetObservedState(observed map[resource.Name]resource.ObservedComposed) error {
	ns := &corev1.Namespace{}

	err := f.svc.GetObservedKubeObject(ns, "namespace")
	if err != nil {
		return err
	}

	f.observed.Namespace = ns

	configMap := &corev1.ConfigMap{}

	err = f.svc.GetObservedKubeObject(configMap, "configmap")
	if err != nil {
		return err
	}

	f.observed.Configmap = configMap

	return nil
}

func test(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {

	svc.GetObservedComposite(comp)

	state := &forgejoState{
		svc: svc,
	}

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

	svc.ObserveState(state)

	fmt.Println(state.observed.Namespace)

	fmt.Println(state.observed.Configmap)

	return nil
}
