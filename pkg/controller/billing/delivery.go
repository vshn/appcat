package billing

import (
	"context"
	"time"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/odoo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// processQueue delivers at most one event per call with priority: resend, failed, pending deletes, any pending.
func (b *BillingHandler) processQueue(ctx context.Context, billingService *vshnv1.BillingService) (bool, error) {
	if i, _, ok := findEvent(billingService, findEventOpts{
		States: []BillingEventState{BillingEventStateResend},
	}); ok {
		return b.deliverQueuedEvent(ctx, billingService, i)
	}

	if i, _, ok := findEvent(billingService, findEventOpts{
		States: []BillingEventState{BillingEventStateFailed},
	}); ok {
		return b.deliverQueuedEvent(ctx, billingService, i)
	}

	delType := BillingEventTypeDeleted
	if i, _, ok := findEvent(billingService, findEventOpts{
		States: []BillingEventState{BillingEventStatePending},
		Type:   &delType,
	}); ok {
		return b.deliverQueuedEvent(ctx, billingService, i)
	}

	if i, _, ok := findEvent(billingService, findEventOpts{
		States: []BillingEventState{BillingEventStatePending},
	}); ok {
		return b.deliverQueuedEvent(ctx, billingService, i)
	}

	return false, nil
}

// deliverQueuedEvent sends the event at idx and updates its state.
func (b *BillingHandler) deliverQueuedEvent(ctx context.Context, billingService *vshnv1.BillingService, idx int) (bool, error) {
	event := billingService.Status.Events[idx]
	event.LastAttemptTime = metav1.Now()

	if err := b.sendEventToOdoo(ctx, billingService, event); err != nil {
		event.RetryCount++
		event.State = string(BillingEventStateFailed)
		billingService.Status.Events[idx] = event
		_ = b.Status().Update(ctx, billingService)
		return false, err
	}

	event.State = string(BillingEventStateSent)
	billingService.Status.Events[idx] = event
	if err := b.Status().Update(ctx, billingService); err != nil {
		return true, err
	}
	return true, nil
}

func (b *BillingHandler) sendEventToOdoo(ctx context.Context, billingService *vshnv1.BillingService, event vshnv1.BillingEventStatus) error {
	// Find the item for this event to get its Unit
	var itemUnit string
	for _, item := range billingService.Spec.Odoo.Items {
		if item.ProductID == event.ProductID {
			itemUnit = item.Unit
			break
		}
	}

	return b.odooClient.SendInstanceEvent(ctx, odoo.InstanceEvent{
		ProductID:            event.ProductID,
		InstanceID:           billingService.Spec.Odoo.InstanceID,
		SalesOrderID:         billingService.Spec.Odoo.SalesOrderID,
		ItemDescription:      billingService.Spec.Odoo.ItemDescription,
		ItemGroupDescription: billingService.Spec.Odoo.ItemGroupDescription,
		UnitID:               itemUnit,
		EventType:            event.Type,
		Size:                 event.Value, // "Size" is Odoo API field name, but we use Value internally
		Timestamp:            event.Timestamp.Format(time.RFC3339),
	})
}
