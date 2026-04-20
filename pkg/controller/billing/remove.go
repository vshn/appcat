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

	// Single pass through events (newest-first order) to find the most recent sent
	// created/scaled event per instanceID. Using sent events as the source of truth
	// ensures we also catch instances whose create event was pruned but scale events remain.
	lastSent := make(map[string]vshnv1.ItemSpec)
	createdInstances := make(map[string]bool)
	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeCreated) && event.State != string(BillingEventStateSuperseded) {
			createdInstances[event.InstanceID] = true
		}
		if _, seen := lastSent[event.InstanceID]; !seen &&
			event.State == string(BillingEventStateSent) &&
			(event.Type == string(BillingEventTypeCreated) || event.Type == string(BillingEventTypeScaled)) {
			lastSent[event.InstanceID] = vshnv1.ItemSpec{
				ProductID:            event.ProductID,
				InstanceID:           event.InstanceID,
				Value:                event.Value,
				ItemDescription:      event.ItemDescription,
				ItemGroupDescription: event.ItemGroupDescription,
			}
		}
	}

	for instanceID := range createdInstances {
		if !currentInstances[instanceID] && !hasEventByInstanceID(billingService, BillingEventTypeDeleted, instanceID) {
			if _, sent := lastSent[instanceID]; !sent {
				b.log.Info("Skipping delete event for instance never sent to Odoo",
					"instanceID", instanceID,
					"billingService", billingService.Name)
			}
		}
	}

	for instanceID, info := range lastSent {
		if currentInstances[instanceID] || hasEventByInstanceID(billingService, BillingEventTypeDeleted, instanceID) {
			continue
		}
		delEvent := vshnv1.BillingEventStatus{
			Type:                 string(BillingEventTypeDeleted),
			ProductID:            info.ProductID,
			InstanceID:           instanceID,
			Value:                info.Value,
			ItemDescription:      info.ItemDescription,
			ItemGroupDescription: info.ItemGroupDescription,
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
