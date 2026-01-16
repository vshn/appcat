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

	delTime := *billingService.DeletionTimestamp

	// Delete all items/products currently in spec
	for _, item := range billingService.Spec.Odoo.Items {
		if !hasEvent(billingService, BillingEventTypeDeleted, item.ProductID) {
			lastValue := lastObservedValueForProduct(billingService, item.ProductID, item.Value)
			event := vshnv1.BillingEventStatus{
				Type:                 string(BillingEventTypeDeleted),
				ProductID:            item.ProductID,
				Value:                lastValue,
				Unit:                 item.Unit,
				ItemDescription:      item.ItemDescription,
				ItemGroupDescription: item.ItemGroupDescription,
				Timestamp:            delTime,
				State:                string(BillingEventStatePending),
				RetryCount:           0,
			}
			if err := enqueueEvent(ctx, b, billingService, event); err != nil {
				return err
			}
		}
	}

	// Finalizer removal is performed in Reconcile once the delete events are sent
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
