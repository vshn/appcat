package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleSLAChange enqueues delete and create events if product changed.
func (b *BillingHandler) handleSLAChange(ctx context.Context, billingService *vshnv1.BillingService) error {
	currentProductID := billingService.Spec.Odoo.ProductID
	currentSize := billingService.Spec.Odoo.Size

	activeProd, activeSize, hasActive := lastActiveSentProduct(billingService)
	if hasActive && activeProd == currentProductID {
		return nil
	}

	if supersedeCreatedForSLA(billingService, currentProductID) {
		if err := b.Status().Update(ctx, billingService); err != nil {
			return err
		}
	}

	now := metav1.Now()

	if hasActive && activeProd != currentProductID && !hasEvent(billingService, BillingEventTypeDeleted, activeProd) {
		delEvent := vshnv1.BillingEventStatus{
			Type:       string(BillingEventTypeDeleted),
			ProductID:  activeProd,
			Size:       lastObservedSizeForProduct(billingService, activeProd, activeSize),
			Timestamp:  now,
			State:      string(BillingEventStatePending),
			RetryCount: 0,
		}
		if err := enqueueEvent(ctx, b, billingService, delEvent); err != nil {
			return err
		}
	}

	if !hasOpenCreated(billingService, currentProductID) {
		createEvent := vshnv1.BillingEventStatus{
			Type:       string(BillingEventTypeCreated),
			ProductID:  currentProductID,
			Size:       currentSize,
			Timestamp:  now,
			State:      string(BillingEventStatePending),
			RetryCount: 0,
		}
		if err := enqueueEvent(ctx, b, billingService, createEvent); err != nil {
			return err
		}
	}

	return nil
}
