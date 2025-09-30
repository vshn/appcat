package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *BillingHandler) handleDeletion(ctx context.Context, billingService *vshnv1.BillingService) error {
	if billingService.DeletionTimestamp.IsZero() {
		return nil
	}

	currentProduct := billingService.Spec.Odoo.ProductID

	if !hasEvent(billingService, BillingEventTypeDeleted, currentProduct) {
		delTime := *billingService.DeletionTimestamp
		lastSize := lastObservedSizeForProduct(billingService, currentProduct, billingService.Spec.Odoo.Size)
		ev := vshnv1.BillingEventStatus{
			Type:       string(BillingEventTypeDeleted),
			ProductID:  currentProduct,
			Size:       lastSize,
			Timestamp:  delTime,
			State:      string(BillingEventStatePending),
			RetryCount: 0,
		}
		if err := enqueueEvent(ctx, b, billingService, ev); err != nil {
			return err
		}
	}

	// Finalizer removal is performed in Reconcile once the delete event is sent
	_ = controllerutil.AddFinalizer(billingService, vshnv1.BillingServiceFinalizer)
	return nil
}
