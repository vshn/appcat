package noop

import (
	"context"
	"fmt"
	"strings"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Noop implements a noop handler for the APIserver.
type Noop struct {
	singular runtime.Object
	list     runtime.Object
	gr       schema.GroupResource
}

var _ rest.Creater = &Noop{}

func New(s *runtime.Scheme, singular runtime.Object, list runtime.Object) *Noop {
	_ = appcatv1.AddToScheme(s)
	return &Noop{
		singular: singular,
		list:     list,
		gr:       schema.GroupResource{Group: singular.GetObjectKind().GroupVersionKind().Group, Resource: singular.GetObjectKind().GroupVersionKind().Kind},
	}
}

func (v Noop) Destroy() {}

func (v Noop) New() runtime.Object {
	return v.singular
}

func (v Noop) Create(_ context.Context, _ runtime.Object, _ rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	return nil, apierrors.NewForbidden(v.gr, "not implemented", fmt.Errorf("method not implemented"))
}

var _ rest.GracefulDeleter = &Noop{}
var _ rest.CollectionDeleter = &Noop{}

func (v Noop) Delete(_ context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return v.singular, false, nil
}

func (v *Noop) DeleteCollection(ctx context.Context, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions, _ *metainternalversion.ListOptions) (runtime.Object, error) {
	return v.singular, nil
}

var _ rest.Getter = &Noop{}

func (v *Noop) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	return v.singular, nil
}

var _ rest.Lister = &Noop{}

func (v *Noop) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return v.list, nil
}

func (v *Noop) NewList() runtime.Object {
	return v.singular
}

var _ rest.TableConvertor = &Noop{}

func (v *Noop) ConvertToTable(_ context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return &metav1.Table{}, nil
}

var _ rest.Updater = &Noop{}
var _ rest.CreaterUpdater = &Noop{}

func (v Noop) Update(_ context.Context, name string, _ rest.UpdatedObjectInfo, _ rest.ValidateObjectFunc, _ rest.ValidateObjectUpdateFunc, _ bool, _ *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return v.list, false, apierrors.NewForbidden(v.gr, "not implemented", fmt.Errorf("method not implemented"))
}

var _ rest.Watcher = &Noop{}

func (v *Noop) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return nil, apierrors.NewForbidden(v.gr, "not implemented", fmt.Errorf("method not implemented"))
}

var _ rest.Scoper = &Noop{}
var _ rest.Storage = &Noop{}

func (v *Noop) NamespaceScoped() bool {
	return true
}

// GetSingularName is needed for the OpenAPI Registartion
func (v *Noop) GetSingularName() string {
	return strings.ToLower(v.singular.GetObjectKind().GroupVersionKind().Kind)
}
