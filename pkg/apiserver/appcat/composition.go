package appcat

import (
	"context"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// compositionProvider is an abstraction to interact with the K8s API
type compositionProvider interface {
	GetComposition(ctx context.Context, name string, options *metav1.GetOptions) (*v1.Composition, error)
	ListCompositions(ctx context.Context, options *metainternalversion.ListOptions) (*v1.CompositionList, error)
	WatchCompositions(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
}

type kubeCompositionProvider struct {
	Client client.WithWatch
}

func (k *kubeCompositionProvider) GetComposition(ctx context.Context, name string, options *metav1.GetOptions) (*v1.Composition, error) {
	c := v1.Composition{}
	err := k.Client.Get(ctx, client.ObjectKey{Namespace: "", Name: name}, &c)
	return &c, err
}

func (k *kubeCompositionProvider) ListCompositions(ctx context.Context, options *metainternalversion.ListOptions) (*v1.CompositionList, error) {
	cl := v1.CompositionList{}
	err := k.Client.List(ctx, &cl, &client.ListOptions{
		LabelSelector: options.LabelSelector,
		FieldSelector: options.FieldSelector,
		Limit:         options.Limit,
		Continue:      options.Continue,
	})
	if err != nil {
		return nil, err
	}
	return &cl, nil
}

func (k *kubeCompositionProvider) WatchCompositions(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	cl := v1.CompositionList{}
	return k.Client.Watch(ctx, &cl, &client.ListOptions{
		LabelSelector: options.LabelSelector,
		FieldSelector: options.FieldSelector,
		Limit:         options.Limit,
		Continue:      options.Continue,
	})
}
