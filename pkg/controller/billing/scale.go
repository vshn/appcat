package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleScaling enqueues a scaled event if the size changed from the last sent value.
func (b *BillingHandler) handleScaling(ctx context.Context, billingService *vshnv1.BillingService) error {
	currentProduct := billingService.Spec.Odoo.ProductID
	currentSize := billingService.Spec.Odoo.Size

	lastSize, ok := lastSentSizeForProduct(billingService, currentProduct)
	if !ok || lastSize == currentSize {
		return nil
	}
	if hasEventWithSize(billingService, BillingEventTypeScaled, currentProduct, currentSize) {
		return nil // already queued
	}

	ev := vshnv1.BillingEventStatus{
		Type:       string(BillingEventTypeScaled),
		ProductID:  currentProduct,
		Size:       currentSize,
		Timestamp:  metav1.Now(),
		State:      string(BillingEventStatePending),
		RetryCount: 0,
	}

	return enqueueEvent(ctx, b, billingService, ev)
}
