package appcat

import (
	"appcat-apiserver/apis/appcat/v1"
	"context"
	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Lister = &appcatStorage{}

func (s *appcatStorage) NewList() runtime.Object {
	return &v1.AppCatList{}
}

// List returns a list of AppCat services based on their compositions
func (s *appcatStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	cl, err := s.compositions.ListCompositions(ctx, addOfferedLabelSelector(options))
	if err != nil {
		return nil, convertCompositionError(err)
	}

	res := v1.AppCatList{
		ListMeta: cl.ListMeta,
	}

	for _, v := range cl.Items {
		appCat := v1.NewAppCatFromComposition(&v)
		if appCat != nil {
			res.Items = append(res.Items, *appCat)
		}
	}

	return &res, nil
}

var _ rest.Watcher = &appcatStorage{}

// Watch returns a watched list of AppCat services based on their compositions
func (s *appcatStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	compWatcher, err := s.compositions.WatchCompositions(ctx, addOfferedLabelSelector(options))
	if err != nil {
		return nil, convertCompositionError(err)
	}

	return watch.Filter(compWatcher, func(in watch.Event) (out watch.Event, keep bool) {
		if in.Object == nil {
			// This should never happen, let downstream deal with it
			return in, true
		}
		comp, ok := in.Object.(*crossplanev1.Composition)
		if !ok {
			// We received a non Composition object
			// This is most likely an error so we pass it on
			return in, true
		}

		in.Object = v1.NewAppCatFromComposition(comp)
		if in.Object.(*v1.AppCat) == nil {
			return in, false
		}

		return in, true
	}), nil
}

func addOfferedLabelSelector(options *metainternalversion.ListOptions) *metainternalversion.ListOptions {
	offeredComposition, err := labels.NewRequirement(v1.OfferedKey, selection.Equals, []string{v1.OfferedValue})
	if err != nil {
		// The input is static. This call will only fail during development.
		panic(err)
	}
	if options == nil {
		options = &metainternalversion.ListOptions{}
	}
	if options.LabelSelector == nil {
		options.LabelSelector = labels.NewSelector()
	}
	options.LabelSelector = options.LabelSelector.Add(*offeredComposition)

	return options
}
