package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleRemovedItems detects items removed from spec and enqueues delete events
func (b *BillingHandler) handleRemovedItems(ctx context.Context, billingService *vshnv1.BillingService) error {
	// Build set of current product IDs in spec
	currentProducts := make(map[string]bool)
	for _, item := range billingService.Spec.Odoo.Items {
		currentProducts[item.ProductID] = true
	}

	// Find products that have created events but are no longer in spec
	seenProducts := make(map[string]bool)
	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeCreated) &&
			event.State != string(BillingEventStateSuperseded) {
			seenProducts[event.ProductID] = true
		}
	}

	// For each item/product removed from spec, enqueue delete event
	for productID := range seenProducts {
		if !currentProducts[productID] {
			if !hasEvent(billingService, BillingEventTypeDeleted, productID) {
				lastValue := lastObservedValueForProduct(billingService, productID, "")
				delEvent := vshnv1.BillingEventStatus{
					Type:       string(BillingEventTypeDeleted),
					ProductID:  productID,
					Value:      lastValue,
					Timestamp:  metav1.Now(),
					State:      string(BillingEventStatePending),
					RetryCount: 0,
				}
				if err := enqueueEvent(ctx, b, billingService, delEvent); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
