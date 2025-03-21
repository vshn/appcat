package redis

import (
	"context"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnredis,verbs=get;list;watch

type vshnRedisProvider interface {
	ListVSHNRedis(ctx context.Context, namespace string) (*vshnv1.VSHNRedisList, error)
	GetKubeConfig(ctx context.Context, instance vshnv1.VSHNRedis) ([]byte, error)
	GetKubeClient(ctx context.Context, instance vshnv1.VSHNRedis) (client.WithWatch, error)
}

type concreteRedisProvider struct {
	client client.Client
}

func (c *concreteRedisProvider) ListVSHNRedis(ctx context.Context, namespace string) (*vshnv1.VSHNRedisList, error) {

	instances := &vshnv1.VSHNRedisList{}

	err := c.client.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNRedis, 0)
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

func (c *concreteRedisProvider) GetKubeConfig(ctx context.Context, instance vshnv1.VSHNRedis) ([]byte, error) {
	providerConfigName := instance.GetLabels()[runtime.ProviderConfigLabel]

	providerConfig := xkube.ProviderConfig{}
	err := c.client.Get(ctx, client.ObjectKey{Name: providerConfigName}, &providerConfig)
	if err != nil {
		return []byte{}, err
	}

	secretRef := providerConfig.Spec.Credentials.SecretRef
	secret := v1.Secret{}
	err = c.client.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret)
	if err != nil {
		return []byte{}, err
	}

	kubeconfig := secret.Data[secretRef.Key]

	return kubeconfig, nil
}

func (c *concreteRedisProvider) GetKubeClient(ctx context.Context, instance vshnv1.VSHNRedis) (client.WithWatch, error) {
	if instance.GetLabels()[runtime.ProviderConfigLabel] == "" {
		return nil, nil
	}

	kubeconfig, err := c.GetKubeConfig(ctx, instance)
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
