package vshnminio

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	minioproviderv1 "github.com/vshn/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployMinioProviderConfig will deploy a providerconfig with the claim's name
func DeployMinioProviderConfig(_ context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	if !svc.GetBoolFromCompositionConfig("providerEnabled") {
		return nil
	}

	comp := &vshnv1.VSHNMinio{}

	err := svc.GetObservedComposite(comp)
	if err != nil {
		err = fmt.Errorf("cannot get desired composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	config := &minioproviderv1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetLabels()["crossplane.io/claim-name"],
		},
		Spec: minioproviderv1.ProviderConfigSpec{
			MinioURL: "http://" + comp.GetName() + "." + comp.GetInstanceNamespace() + ".svc:9000/",
			Credentials: minioproviderv1.ProviderCredentials{
				APISecretRef: corev1.SecretReference{
					Name:      comp.GetName() + "-connection",
					Namespace: comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err = svc.SetDesiredKubeObject(config, comp.GetName()+"-providerconfig")
	if err != nil {
		err = fmt.Errorf("cannot get providerconfig: %w", err)
		return runtime.NewFatalResult(err)
	}

	return nil
}
