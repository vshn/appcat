package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleItemScaling enqueues a scaled event if the value changed from the last sent value.
func (b *BillingHandler) handleItemScaling(ctx context.Context, billingService *vshnv1.BillingService, item vshnv1.ItemSpec) error {
	lastValue, ok := lastSentValueForProduct(billingService, item.ProductID)
	if !ok || lastValue == item.Value {
		return nil
	}
	if hasEventWithValue(billingService, BillingEventTypeScaled, item.ProductID, item.Value) {
		return nil // already queued
	}

	ev := vshnv1.BillingEventStatus{
		Type:       string(BillingEventTypeScaled),
		ProductID:  item.ProductID,
		Value:      item.Value,
		Timestamp:  metav1.Now(),
		State:      string(BillingEventStatePending),
		RetryCount: 0,
	}

	return enqueueEvent(ctx, b, billingService, ev)
}
