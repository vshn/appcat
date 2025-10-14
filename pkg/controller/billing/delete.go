package billing

import (
	"context"
	"time"

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

// shouldRemoveFinalizer determines if the finalizer can be removed based on whether the KeepAfterDeletion grace period has elapsed
func shouldRemoveFinalizer(billingService *vshnv1.BillingService) (bool, *time.Duration) {

	// If KeepAfterDeletion is not set or is 0, allow immediate deletion
	keepAfterDeletion := billingService.Spec.KeepAfterDeletion
	if keepAfterDeletion <= 0 {
		return true, nil
	}

	// Calculate the time when deletion is allowed
	deletionTimestamp := billingService.DeletionTimestamp.Time
	allowedDeletionTime := deletionTimestamp.Add(time.Duration(keepAfterDeletion) * 24 * time.Hour)
	now := time.Now()

	// If the grace period has elapsed, allow deletion
	if now.After(allowedDeletionTime) || now.Equal(allowedDeletionTime) {
		return true, nil
	}

	// Calculate time until deletion is allowed and schedule requeue
	remainingTime := allowedDeletionTime.Sub(now)
	return false, &remainingTime
}
