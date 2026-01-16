package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleRemovedItems detects items removed from spec and enqueues delete events
func (b *BillingHandler) handleRemovedItems(ctx context.Context, billingService *vshnv1.BillingService) error {
	currentProducts := make(map[string]bool)
	for _, item := range billingService.Spec.Odoo.Items {
		currentProducts[item.ProductID] = true
	}

	// Single pass through events (newest-first order)
	createdProducts := make(map[string]bool) // products with non-superseded created events
	lastSentValue := make(map[string]string) // most recent sent value per product
	lastSentUnit := make(map[string]string)  // most recent sent unit per product

	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeCreated) &&
			event.State != string(BillingEventStateSuperseded) {
			createdProducts[event.ProductID] = true
		}

		// Capture from first (most recent) sent created/scaled event
		if _, seen := lastSentValue[event.ProductID]; !seen &&
			event.State == string(BillingEventStateSent) &&
			(event.Type == string(BillingEventTypeCreated) || event.Type == string(BillingEventTypeScaled)) {
			lastSentValue[event.ProductID] = event.Value
			lastSentUnit[event.ProductID] = event.Unit
		}
	}

	for productID := range createdProducts {
		if currentProducts[productID] || hasEvent(billingService, BillingEventTypeDeleted, productID) {
			continue
		}

		delEvent := vshnv1.BillingEventStatus{
			Type:       string(BillingEventTypeDeleted),
			ProductID:  productID,
			Value:      lastSentValue[productID],
			Unit:       lastSentUnit[productID],
			Timestamp:  metav1.Now(),
			State:      string(BillingEventStatePending),
			RetryCount: 0,
		}
		if err := enqueueEvent(ctx, b, billingService, delEvent); err != nil {
			return err
		}
	}

	return nil
}
