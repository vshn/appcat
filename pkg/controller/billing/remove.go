package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleRemovedItems detects items removed from spec and enqueues delete events
func (b *BillingHandler) handleRemovedItems(ctx context.Context, billingService *vshnv1.BillingService) error {
	currentInstances := make(map[string]bool)
	for _, item := range billingService.Spec.Odoo.Items {
		currentInstances[item.InstanceID] = true
	}

	// Single pass through events (newest-first order), keyed by instanceID
	type eventInfo struct {
		productID, value, itemDesc, itemGroupDesc string
	}
	createdInstances := make(map[string]bool)
	lastSent := make(map[string]eventInfo)

	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeCreated) &&
			event.State != string(BillingEventStateSuperseded) {
			createdInstances[event.InstanceID] = true
		}

		// Capture from first (most recent) sent created/scaled event per instanceID
		if _, seen := lastSent[event.InstanceID]; !seen &&
			event.State == string(BillingEventStateSent) &&
			(event.Type == string(BillingEventTypeCreated) || event.Type == string(BillingEventTypeScaled)) {
			lastSent[event.InstanceID] = eventInfo{
				productID:     event.ProductID,
				value:         event.Value,
				itemDesc:      event.ItemDescription,
				itemGroupDesc: event.ItemGroupDescription,
			}
		}
	}

	for instanceID := range createdInstances {
		if currentInstances[instanceID] || hasEventByInstanceID(billingService, BillingEventTypeDeleted, instanceID) {
			continue
		}

		info, ok := lastSent[instanceID]
		if !ok {
			// Never sent to Odoo — nothing to delete
			b.log.Info("Skipping delete event for instance never sent to Odoo",
				"instanceID", instanceID,
				"billingService", billingService.Name)
			continue
		}
		delEvent := vshnv1.BillingEventStatus{
			Type:                 string(BillingEventTypeDeleted),
			ProductID:            info.productID,
			InstanceID:           instanceID,
			Value:                info.value,
			ItemDescription:      info.itemDesc,
			ItemGroupDescription: info.itemGroupDesc,
			Timestamp:            metav1.Now(),
			State:                string(BillingEventStatePending),
			RetryCount:           0,
		}
		if err := enqueueEvent(ctx, b, billingService, delEvent); err != nil {
			return err
		}
	}

	return nil
}
