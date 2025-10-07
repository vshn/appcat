package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func (b *BillingHandler) handleCreation(ctx context.Context, billingService *vshnv1.BillingService) error {
	currentProd := billingService.Spec.Odoo.ProductID
	currentSize := billingService.Spec.Odoo.Size

	if hasEvent(billingService, BillingEventTypeCreated, currentProd) {
		return nil
	}

	event := vshnv1.BillingEventStatus{
		Type:       string(BillingEventTypeCreated),
		ProductID:  currentProd,
		Size:       currentSize,
		Timestamp:  billingService.ObjectMeta.CreationTimestamp,
		State:      string(BillingEventStatePending),
		RetryCount: 0,
	}

	return enqueueEvent(ctx, b, billingService, event)
}
