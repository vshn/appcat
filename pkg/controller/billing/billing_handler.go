package billing

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=billingservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=billingservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=billingservices/finalizers,verbs=update

import (
	"context"

	"github.com/go-logr/logr"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/odoo"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type BillingHandler struct {
	client.Client
	Scheme     *runtime.Scheme
	odooClient *odoo.Client
	log        logr.Logger
}

func New(c client.Client, scheme *runtime.Scheme, odooClient *odoo.Client) *BillingHandler {
	return &BillingHandler{
		Client:     c,
		Scheme:     scheme,
		odooClient: odooClient,
		log:        ctrl.Log.WithName("controller").WithName("billing"),
	}
}

func (b *BillingHandler) SetupWithManager(mgr ctrl.Manager) error {
	updateOnDeletionOrResend := predicate.Funcs{
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldDel := event.ObjectOld.GetDeletionTimestamp() != nil
			newDel := event.ObjectNew.GetDeletionTimestamp() != nil
			if !oldDel && newDel {
				return true
			}
			oldAnn := event.ObjectOld.GetAnnotations()
			newAnn := event.ObjectNew.GetAnnotations()
			var oldVal, newVal string
			if oldAnn != nil {
				oldVal = oldAnn[ResendAnnotationKey]
			}
			if newAnn != nil {
				newVal = newAnn[ResendAnnotationKey]
			}
			if oldVal != newVal {
				return true
			}
			return event.ObjectOld.GetGeneration() != event.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(event.DeleteEvent) bool { return true },
	}

	rateLimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](minBackoff, maxBackoff),
		&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.BillingService{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, updateOnDeletionOrResend),
		)).
		WithOptions(controller.Options{RateLimiter: rateLimiter}).
		Complete(b)
}

func (b *BillingHandler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var billingService vshnv1.BillingService
	if err := b.Get(ctx, req.NamespacedName, &billingService); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if billingService.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(&billingService, vshnv1.BillingServiceFinalizer) {
		if err := b.Update(ctx, &billingService); err != nil {
			return ctrl.Result{}, err
		}
	}

	if mode, ok := billingService.Annotations[ResendAnnotationKey]; ok && mode != "" {
		if handleResendAnnotation(&billingService, mode) {
			if err := b.Status().Update(ctx, &billingService); err != nil {
				return ctrl.Result{}, err
			}
		}
		delete(billingService.Annotations, ResendAnnotationKey)
		if err := b.Update(ctx, &billingService); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !billingService.DeletionTimestamp.IsZero() {
		if err := b.handleDeletion(ctx, &billingService); err != nil {
			return ctrl.Result{}, err
		}

		sent, err := b.processQueue(ctx, &billingService)
		if err != nil {
			_ = b.updateStatus(ctx, &billingService)
			return ctrl.Result{}, err
		}

		if hasSentEvent(&billingService, BillingEventTypeDeleted, billingService.Spec.Odoo.ProductID, "") &&
			controllerutil.ContainsFinalizer(&billingService, vshnv1.BillingServiceFinalizer) {
			controllerutil.RemoveFinalizer(&billingService, vshnv1.BillingServiceFinalizer)
			if err := b.Update(ctx, &billingService); err != nil {
				return ctrl.Result{}, err
			}
		}

		if err := b.updateStatus(ctx, &billingService); err != nil {
			return ctrl.Result{}, err
		}

		if sent || hasBacklog(&billingService) {
			return ctrl.Result{RequeueAfter: successDrainDelay}, nil
		}

		return ctrl.Result{}, nil
	}

	if err := b.handleSLAChange(ctx, &billingService); err != nil {
		return ctrl.Result{}, err
	}

	if err := b.handleCreation(ctx, &billingService); err != nil {
		return ctrl.Result{}, err
	}

	if err := b.handleScaling(ctx, &billingService); err != nil {
		return ctrl.Result{}, err
	}

	sent, err := b.processQueue(ctx, &billingService)
	if err != nil {
		_ = b.updateStatus(ctx, &billingService)
		return ctrl.Result{}, err
	}

	if err := b.updateStatus(ctx, &billingService); err != nil {
		return ctrl.Result{}, err
	}

	if sent || hasBacklog(&billingService) {
		return ctrl.Result{RequeueAfter: successDrainDelay}, nil
	}
	return ctrl.Result{}, nil
}
