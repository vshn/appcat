package billing

import (
	"context"
	"fmt"
	"time"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/odoo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	var fresh *vshnv1.BillingService
	if err := retry.OnError(retry.DefaultRetry, isTransientAPIError, func() error {
		var err error
		fresh, err = b.persistEventSent(ctx, billingService, event)
		return err
	}); err != nil {
		return true, err
	}
	*billingService = *fresh
	return true, nil
}

// persistEventSent re-fetches billingService from the cluster, marks the event matching
// event.InstanceID as sent, and persists the status. Returns the updated object on success.
func (b *BillingHandler) persistEventSent(ctx context.Context, billingService *vshnv1.BillingService, event vshnv1.BillingEventStatus) (*vshnv1.BillingService, error) {
	fresh := &vshnv1.BillingService{}
	if err := b.Get(ctx, client.ObjectKeyFromObject(billingService), fresh); err != nil {
		return nil, err
	}
	found := false
	for i, e := range fresh.Status.Events {
		if e.InstanceID == event.InstanceID && e.Type == event.Type && e.Timestamp.Time.Equal(event.Timestamp.Time) {
			fresh.Status.Events[i].State = string(BillingEventStateSent)
			fresh.Status.Events[i].LastAttemptTime = event.LastAttemptTime
			found = true
			break
		}
	}
	if !found {
		// Event was pruned between Odoo delivery and this status persist. This is a known
		// rare race: the next reconcile will re-enqueue the event as pending and re-send it
		// to Odoo. Odoo deduplicates duplicate events on its side, so this is acceptable.
		return nil, fmt.Errorf("event for instanceID %q type %q not found after re-fetch (pruned?)", event.InstanceID, event.Type)
	}
	return fresh, b.Status().Update(ctx, fresh)
}

// isTransientAPIError returns true for transient k8s API errors that are safe to retry.
func isTransientAPIError(err error) bool {
	return apierrors.IsConflict(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err)
}

func (b *BillingHandler) sendEventToOdoo(ctx context.Context, billingService *vshnv1.BillingService, event vshnv1.BillingEventStatus) error {
	if event.InstanceID == "" {
		return fmt.Errorf("event for product %q on %s/%s has no instanceID", event.ProductID, billingService.Namespace, billingService.Name)
	}
	return b.odooClient.SendInstanceEvent(ctx, odoo.InstanceEvent{
		ProductID:            event.ProductID,
		InstanceID:           event.InstanceID,
		SalesOrderID:         billingService.Spec.Odoo.SalesOrderID,
		ItemDescription:      event.ItemDescription,
		ItemGroupDescription: event.ItemGroupDescription,
		EventType:            event.Type,
		Size:                 event.Value,
		Timestamp:            event.Timestamp.Format(time.RFC3339),
	})
}
