package pendingfixer

import (
	"context"
	"time"

	xpmetav1 "github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/go-logr/logr"
	xhelm "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ObjectFixer struct {
	c   client.Client
	log logr.Logger
}

func SetupWithManagerObject(mgr ctrl.Manager) error {
	r := &ObjectFixer{
		c: mgr.GetClient(),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&xkube.Object{}).
		Named("PendingObjectFixer").
		// We only react to annotation changes
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		Complete(r)
}

func (p *ObjectFixer) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	obj := &xkube.Object{}

	err := p.c.Get(ctx, request.NamespacedName, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, err
	}

	if _, ok := obj.GetAnnotations()[xpmetav1.AnnotationKeyExternalCreatePending]; ok {
		annotations := obj.GetAnnotations()
		delete(annotations, xpmetav1.AnnotationKeyExternalCreatePending)
		// Some providers filter out the pending annotation in their controller events,
		// so we need to set another annotation to actually trigger the
		// next reconcile.
		annotations["appcat.vshn.io/pending-fixer"] = "fixed " + time.Now().Format(time.RFC3339)
		obj.SetAnnotations(annotations)
		ctrl.LoggerFrom(ctx).Info("removing pending annotation", "name", request.Name)

		return ctrl.Result{}, p.c.Update(ctx, obj)
	}

	return ctrl.Result{}, nil
}

type ReleaseFixer struct {
	c client.Client
}

func SetupWithManagerRelease(mgr ctrl.Manager) error {
	r := &ObjectFixer{
		c: mgr.GetClient(),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&xhelm.Release{}).
		Named("PendingReleaseFixer").
		// We only react to annotation changes
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		Complete(r)
}

func (r *ReleaseFixer) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	obj := &xhelm.Release{}

	err := r.c.Get(ctx, request.NamespacedName, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, err
	}

	if _, ok := obj.GetAnnotations()[xpmetav1.AnnotationKeyExternalCreatePending]; ok {
		annotations := obj.GetAnnotations()
		delete(annotations, xpmetav1.AnnotationKeyExternalCreatePending)
		// Some providers filter out the pending annotation in their controller events,
		// so we need to set another annotation to actually trigger the
		// next reconcile.
		annotations["appcat.vshn.io/pending-fixer"] = "fixed " + time.Now().Format(time.RFC3339)
		obj.SetAnnotations(annotations)
		ctrl.LoggerFrom(ctx).Info("removing pending annotation", "name", request.Name)

		return ctrl.Result{}, r.c.Update(ctx, obj)
	}

	return ctrl.Result{}, nil
}
