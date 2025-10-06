package apiserver

import (
	"context"
	"errors"
	v2 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"sync"

	"github.com/vshn/appcat/v4/pkg"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/v1alpha1"
	appcatruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynClient "k8s.io/client-go/dynamic"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
)

func ResolveError(groupResource schema.GroupResource, err error) error {
	statusErr := &apierrors.StatusError{}

	if errors.As(err, &statusErr) {
		switch {
		case apierrors.IsNotFound(err):
			return apierrors.NewNotFound(groupResource, statusErr.ErrStatus.Details.Name)
		case apierrors.IsAlreadyExists(err):
			return apierrors.NewAlreadyExists(groupResource, statusErr.ErrStatus.Details.Name)
		}
	}
	return err
}

// MultiWatch is wrapper of multiple source watches which implements the same methods as a normal watch.Watch
var _ watch.Interface = &MultiWatcher{}

type MultiWatcher struct {
	watchers  []watch.Interface
	eventChan chan watch.Event
	wg        sync.WaitGroup
}

// NewEmptyMultiWatch creates an empty watch
func NewEmptyMultiWatch() *MultiWatcher {
	return &MultiWatcher{
		eventChan: make(chan watch.Event),
	}
}

// AddWatcher adds a watch to this MultiWatcher
func (m *MultiWatcher) AddWatcher(w watch.Interface) {
	m.watchers = append(m.watchers, w)
}

// Stop stops all watches of this MultiWatch
func (m *MultiWatcher) Stop() {
	for _, watcher := range m.watchers {
		watcher.Stop()
	}
	m.wg.Wait()
	close(m.eventChan)
}

// ResultChan aggregates all channels from all watches of this MultiWatch
func (m *MultiWatcher) ResultChan() <-chan watch.Event {
	for _, w := range m.watchers {
		m.wg.Add(1)
		watcher := w
		go func() {
			defer m.wg.Done()
			for c := range watcher.ResultChan() {
				m.eventChan <- c
			}
		}()
	}
	return m.eventChan
}

func GetBackupTable(id, instance, status, age, started, finished string, backup runtime.Object) metav1.TableRow {
	return metav1.TableRow{
		Cells:  []interface{}{id, instance, started, finished, status, age}, // Snapshots are created only when the backup successfully finished
		Object: runtime.RawExtension{Object: backup},
	}
}

func GetBackupColumnDefinition() []metav1.TableColumnDefinition {
	desc := metav1.ObjectMeta{}.SwaggerDoc()
	return []metav1.TableColumnDefinition{
		{Name: "Backup ID", Type: "string", Format: "name", Description: desc["name"]},
		{Name: "Instance", Type: "string", Description: "The instance that this backup belongs to"},
		{Name: "Started", Type: "string", Description: "The backup start time"},
		{Name: "Finished", Type: "string", Description: "The data is available up to this time"},
		{Name: "Status", Type: "string", Description: "The state of this backup"},
		{Name: "Age", Type: "date", Description: desc["creationTimestamp"]},
	}
}

type ClientConfigurator interface {
	GetDynKubeClient(ctx context.Context, instance client.Object) (*dynClient.DynamicClient, error)
	GetKubeClient(ctx context.Context, instance client.Object) (client.WithWatch, error)
	client.WithWatch
}

type KubeClient struct {
	client.WithWatch
}

func New(client client.WithWatch) *KubeClient {
	return &KubeClient{
		WithWatch: client,
	}
}

// GetKubeClient will return a `Client` for the provided instance and kubeclient
// It will check where the instance is running on and will return either the client
// for the remote cluster (non-converged) or nil for the local cluster
func (k *KubeClient) GetKubeClient(ctx context.Context, instance client.Object) (client.WithWatch, error) {
	if instance.GetLabels()[appcatruntime.ProviderConfigLabel] == "" {
		return k.WithWatch, nil
	}

	providerConfig, err := k.fetchProvider(ctx, instance.GetLabels()[appcatruntime.ProviderConfigLabel])
	if err != nil {
		return nil, err
	}

	kubeconfig, err := k.getKubeConfig(ctx, providerConfig)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := client.NewWithWatch(config, client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return nil, err
	}

	return kubeClient, nil
}

// GetDynKubeClient will return a `DynamicClient` for the provided instance and kubeclient
// It will check where the instance is running on and will return either the client
// for the remote cluster (non-converged) or the local cluster (converged)
func (k *KubeClient) GetDynKubeClient(ctx context.Context, instance client.Object) (*dynClient.DynamicClient, error) {
	providerConfigLabelValue := instance.GetLabels()[appcatruntime.ProviderConfigLabel]
	if providerConfigLabelValue == "" || providerConfigLabelValue == "local" {
		// For converged clusters, create a dynamic client using the loopback config
		return dynClient.NewForConfig(loopback.GetLoopbackMasterClientConfig())
	}

	providerConfig, err := k.fetchProvider(ctx, instance.GetLabels()[appcatruntime.ProviderConfigLabel])
	if err != nil {
		return nil, err
	}

	if providerConfig.Spec.Credentials.Source == v2.CredentialsSourceInjectedIdentity {
		return dynClient.NewForConfig(loopback.GetLoopbackMasterClientConfig())
	}

	kubeconfig, err := k.getKubeConfig(ctx, providerConfig)
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

func (k *KubeClient) fetchProvider(ctx context.Context, providerConfigName string) (*xkube.ProviderConfig, error) {
	providerConfig := &xkube.ProviderConfig{}
	err := k.Get(ctx, client.ObjectKey{Name: providerConfigName}, providerConfig)
	if err != nil {
		return nil, err
	}
	return providerConfig, err
}

// GetKubeConfig will return a `Kubeconfig` for the provided instance and kubeclient
func (k *KubeClient) getKubeConfig(ctx context.Context, providerConfig *xkube.ProviderConfig) ([]byte, error) {
	secretRef := providerConfig.Spec.Credentials.SecretRef
	secret := v1.Secret{}
	err := k.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret)
	if err != nil {
		return []byte{}, err
	}

	kubeconfig := secret.Data[secretRef.Key]
	return kubeconfig, nil
}
