package poctest

import (
	"context"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	aruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type forgejoState struct {
	desired   forgejoObjects
	observed  forgejoObjects
	composite *vshnv1.VSHNForgejo
	svc       *aruntime.ServiceRuntime
}

type forgejoObjects struct {
	// Helm      *xhelm.Release
	Namespace *corev1.Namespace
	// Configmap *corev1.ConfigMap
	// Release   *xhelm.Release
	// Dynamic []client.Object
}

func (f *forgejoState) GetDesiredState() any {
	return f.desired
}

func (f *forgejoState) SetObservedState(observed map[resource.Name]resource.ObservedComposed) error {
	// ns := &corev1.Namespace{}

	// err := f.svc.GetObservedKubeObject(ns, "namespace")
	// if err != nil {
	// 	return err
	// }

	// f.observed.Namespace = ns

	// configMap := &corev1.ConfigMap{}

	// err = f.svc.GetObservedKubeObject(configMap, "configmap")
	// if err != nil {
	// 	return err
	// }

	// f.observed.Configmap = configMap

	return nil
}

func test(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *aruntime.ServiceRuntime) *xfnproto.Result {

	svc.GetObservedComposite(comp)

	state := &forgejoState{
		svc: svc,
	}

	state.desired.Namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec:   corev1.NamespaceSpec{},
		Status: corev1.NamespaceStatus{},
	}

	// state.desired.Configmap = &corev1.ConfigMap{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      comp.GetName(),
	// 		Namespace: comp.GetName(),
	// 	},
	// 	Data: map[string]string{
	// 		"test": "test",
	// 	},
	// }

	// svc.ApplyState(state)

	// svc.ObserveState(state)

	// fmt.Println(state.observed.Namespace)

	// fmt.Println(state.observed.Configmap)

	// vb, err := json.Marshal(
	// 	map[string]string{
	// 		"test": "test",
	// 	},
	// )
	// if err != nil {
	// 	panic(err)
	// }

	// state.desired.Helm = &xhelm.Release{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: "test",
	// 	},
	// 	Spec: xhelm.ReleaseSpec{
	// 		ForProvider: xhelm.ReleaseParameters{
	// 			ValuesSpec: xhelm.ValuesSpec{
	// 				Values: runtime.RawExtension{
	// 					Raw: vb,
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	svc.ApplyState(state)

	for k, v := range svc.GetAllDesired() {
		fmt.Println(k)
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(raw))
	}

	return nil
}
