package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

// handleItemCreation checks if each item/product has a created event
func (b *BillingHandler) handleItemCreation(ctx context.Context, billingService *vshnv1.BillingService, item vshnv1.ItemSpec) error {
	if hasEvent(billingService, BillingEventTypeCreated, item.ProductID) {
		return nil
	}

	event := vshnv1.BillingEventStatus{
		Type:                 string(BillingEventTypeCreated),
		ProductID:            item.ProductID,
		Value:                item.Value,
		Unit:                 item.Unit,
		ItemDescription:      item.ItemDescription,
		ItemGroupDescription: item.ItemGroupDescription,
		Timestamp:            billingService.ObjectMeta.CreationTimestamp,
		State:                string(BillingEventStatePending),
		RetryCount:           0,
	}

	return enqueueEvent(ctx, b, billingService, event)
}
