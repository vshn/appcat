package nextcloud

import (
	"context"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	dynClient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnnextclouds,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnnextclouds,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnpostgresqls,verbs=get;list;watch

type vshnNextcloudProvider interface {
	ListVSHNnextcloud(ctx context.Context, namespace string) (*vshnv1.VSHNNextcloudList, error)
	GetKubeConfig(ctx context.Context, instance vshnv1.VSHNNextcloud) ([]byte, error)
	GetKubeClient(ctx context.Context, instance vshnv1.VSHNNextcloud) (client.WithWatch, error)
	GetDynKubeClient(ctx context.Context, instance vshnv1.VSHNNextcloud) (*dynClient.DynamicClient, error)
}

type concreteNextcloudProvider struct {
	client client.Client
}

func (c *concreteNextcloudProvider) ListVSHNnextcloud(ctx context.Context, namespace string) (*vshnv1.VSHNNextcloudList, error) {

	instances := &vshnv1.VSHNNextcloudList{}

	err := c.client.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNNextcloud, 0)
	for _, p := range instances.Items {
		//
		// In some cases instance namespaces is missing and as a consequence all backups from the whole cluster
		// are being exposed creating a security issue - check APPCAT-563.
		if p.Status.InstanceNamespace != "" {
			cleanedList = append(cleanedList, p)
		}
	}
	instances.Items = cleanedList

	return instances, nil
}

func (k *concreteNextcloudProvider) GetKubeConfig(ctx context.Context, instance vshnv1.VSHNNextcloud) ([]byte, error) {
	providerConfigName := instance.GetLabels()[runtime.ProviderConfigLabel]

	providerConfig := xkube.ProviderConfig{}
	err := k.client.Get(ctx, client.ObjectKey{Name: providerConfigName}, &providerConfig)
	if err != nil {
		return []byte{}, err
	}

	secretRef := providerConfig.Spec.Credentials.SecretRef
	secret := v1.Secret{}
	err = k.client.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret)
	if err != nil {
		return []byte{}, err
	}

	kubeconfig := secret.Data[secretRef.Key]

	return kubeconfig, nil
}

func (k *concreteNextcloudProvider) GetKubeClient(ctx context.Context, instance vshnv1.VSHNNextcloud) (client.WithWatch, error) {
	if instance.GetLabels()[runtime.ProviderConfigLabel] == "" {
		return nil, nil
	}

	kubeconfig, err := k.GetKubeConfig(ctx, instance)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := client.NewWithWatch(config, client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return client, err
	}
	return client, nil
}

func (k *concreteNextcloudProvider) GetDynKubeClient(ctx context.Context, instance vshnv1.VSHNNextcloud) (*dynClient.DynamicClient, error) {
	if instance.GetLabels()[runtime.ProviderConfigLabel] == "" {
		return nil, nil
	}

	kubeconfig, err := k.GetKubeConfig(ctx, instance)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := dynClient.NewForConfig(config)
	if err != nil {
		return client, err
	}
	return client, nil
}
