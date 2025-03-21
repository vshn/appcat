package postgres

import (
	"context"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	dynClient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnpostgresqls,verbs=get;list;watch

// vshnPostgresqlProvider is an abstraction to interact with the K8s API
type vshnPostgresqlProvider interface {
	ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error)
	GetKubeClient(ctx context.Context, instance vshnv1.VSHNPostgreSQL) (*dynClient.DynamicClient, error)
}

type kubeVSHNPostgresqlProvider struct {
	client client.Client
}

// ListXVSHNPostgreSQL fetches a list of XVSHNPostgreSQL.
func (k *kubeVSHNPostgresqlProvider) ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error) {
	list := &vshnv1.VSHNPostgreSQLList{}
	err := k.client.List(ctx, list, &client.ListOptions{Namespace: namespace})
	cleanedList := make([]vshnv1.VSHNPostgreSQL, 0)
	for _, p := range list.Items {
		// In some cases instance namespaces is missing and as a consequence all backups from the whole cluster
		// are being exposed creating a security issue - check APPCAT-563.
		if p.Status.InstanceNamespace != "" {
			cleanedList = append(cleanedList, p)
		}
	}
	list.Items = cleanedList
	return list, err
}

func (k *kubeVSHNPostgresqlProvider) GetKubeConfig(ctx context.Context, instance vshnv1.VSHNPostgreSQL) ([]byte, error) {
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

func (k *kubeVSHNPostgresqlProvider) GetKubeClient(ctx context.Context, instance vshnv1.VSHNPostgreSQL) (*dynClient.DynamicClient, error) {
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
