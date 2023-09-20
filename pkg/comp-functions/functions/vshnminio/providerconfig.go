package vshnminio

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	minioproviderv1 "github.com/vshn/provider-minio/apis/provider/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployMinioProviderConfig will deploy a providerconfig with the claim's name
func DeployMinioProviderConfig(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	if !iof.GetBoolFromCompositionConfig("providerEnabled") {
		return runtime.NewNormal()
	}

	comp := &vshnv1.VSHNMinio{}

	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get composition", err)
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

	err = iof.Desired.PutIntoObject(ctx, config, comp.GetName()+"-providerconfig")
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot write minio providerconfig", err)
	}

	return runtime.NewNormal()
}
