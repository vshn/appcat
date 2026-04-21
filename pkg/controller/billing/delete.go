package billing

import (
	"context"
	"time"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handleDeletion enqueues delete events for every item that was ever sent to Odoo.
//
// It processes two sources to ensure no instance is missed:
//  1. Current spec items — the normal case where all items are present.
//  2. Sent created/scaled events in status — catches items transiently absent from spec.
//     Crossplane applies the composed BillingService in two steps: first sets DeletionTimestamp,
//     then may still update spec via provider-kubernetes. If the spec momentarily has N-1 items
//     while status shows N sent events, relying on spec alone permanently misses the Nth delete.
//     Scaled events are included to handle cases where the original create event was pruned.
func (b *BillingHandler) handleDeletion(ctx context.Context, billingService *vshnv1.BillingService) error {
	if billingService.DeletionTimestamp.IsZero() {
		return nil
	}

	delTime := *billingService.DeletionTimestamp
	seen := make(map[string]bool)

	// Spec items first (normal path).
	for _, item := range billingService.Spec.Odoo.Items {
		if err := b.enqueueDeleteIfNeeded(ctx, billingService, item, delTime, seen); err != nil {
			return err
		}
	}

	// Sent created/scaled events not already covered by spec (transient spec gap).
	// seen prevents double-enqueue when both a create and scale exist for the same instanceID.
	for _, event := range billingService.Status.Events {
		if seen[event.InstanceID] {
			continue
		}
		if event.State != string(BillingEventStateSent) {
			continue
		}
		if event.Type != string(BillingEventTypeCreated) && event.Type != string(BillingEventTypeScaled) {
			continue
		}
		if err := b.enqueueDeleteIfNeeded(ctx, billingService, vshnv1.ItemSpec{
			ProductID:            event.ProductID,
			InstanceID:           event.InstanceID,
			Value:                event.Value,
			ItemDescription:      event.ItemDescription,
			ItemGroupDescription: event.ItemGroupDescription,
		}, delTime, seen); err != nil {
			return err
		}
	}

	_ = controllerutil.AddFinalizer(billingService, vshnv1.BillingServiceFinalizer)
	return nil
}

func (b *BillingHandler) enqueueDeleteIfNeeded(ctx context.Context, billingService *vshnv1.BillingService, it vshnv1.ItemSpec, delTime metav1.Time, seen map[string]bool) error {
	if seen[it.InstanceID] || hasEventByInstanceID(billingService, BillingEventTypeDeleted, it.InstanceID) {
		return nil
	}
	seen[it.InstanceID] = true
	return enqueueEvent(ctx, b, billingService, vshnv1.BillingEventStatus{
		Type:                 string(BillingEventTypeDeleted),
		ProductID:            it.ProductID,
		InstanceID:           it.InstanceID,
		Value:                lastObservedValueForInstanceID(billingService, it.InstanceID, it.Value),
		ItemDescription:      it.ItemDescription,
		ItemGroupDescription: it.ItemGroupDescription,
		Timestamp:            delTime,
		State:                string(BillingEventStatePending),
	})
}

// shouldRemoveFinalizer determines if the finalizer can be removed based on whether the KeepAfterDeletion grace period has elapsed
func shouldRemoveFinalizer(billingService *vshnv1.BillingService) (bool, *time.Duration) {

	// If KeepAfterDeletion is not set or is 0, allow immediate deletion
	keepAfterDeletion := billingService.Spec.KeepAfterDeletion
	if keepAfterDeletion <= 0 {
		return true, nil
	}

	// Calculate the time when deletion is allowed
	deletionTimestamp := billingService.DeletionTimestamp.Time
	allowedDeletionTime := deletionTimestamp.Add(time.Duration(keepAfterDeletion) * 24 * time.Hour)
	now := time.Now()

	// If the grace period has elapsed, allow deletion
	if now.After(allowedDeletionTime) || now.Equal(allowedDeletionTime) {
		return true, nil
	}

	// Calculate time until deletion is allowed and schedule requeue
	remainingTime := allowedDeletionTime.Sub(now)
	return false, &remainingTime
}
