package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

type findEventOpts struct {
	States    []BillingEventState
	Type      *BillingEventType
	ProductID *string
}

// enqueueEvent prepends a billing event to the list and updates the BillingService status.
func enqueueEvent(ctx context.Context, b *BillingHandler, billingService *vshnv1.BillingService, event vshnv1.BillingEventStatus) error {
	// Prepend event (newest first)
	billingService.Status.Events = append([]vshnv1.BillingEventStatus{event}, billingService.Status.Events...)

	// Prune events if needed (per-product limits)
	pruned := pruneEventsIfNeeded(billingService, b.maxEvents)
	if pruned > 0 {
		b.log.Info("Pruned old billing events",
			"billingService", billingService.Name,
			"prunedCount", pruned,
			"remainingEvents", len(billingService.Status.Events))
	}

	return b.Status().Update(ctx, billingService)
}

// pruneEventsIfNeeded removes oldest sent events per product when limit exceeded
// maxEvents applies globally to all products (same limit for each product)
func pruneEventsIfNeeded(billingService *vshnv1.BillingService, maxEvents int) int {
	// Count events per product
	eventCountPerProduct := make(map[string]int)
	for _, event := range billingService.Status.Events {
		eventCountPerProduct[event.ProductID]++
	}

	// Build pruning list per product
	eventsToRemove := make(map[int]bool)
	totalPruned := 0

	// Get all product IDs from items
	productIDs := make(map[string]bool)
	for _, item := range billingService.Spec.Odoo.Items {
		productIDs[item.ProductID] = true
	}

	for productID := range productIDs {
		currentCount := eventCountPerProduct[productID]
		if currentCount <= maxEvents {
			continue // no pruning needed for this product
		}

		toPrune := currentCount - maxEvents

		// Find oldest sent events for this product (from end of array)
		// Only prune events with status "sent"
		prunableIndices := []int{}
		for i := len(billingService.Status.Events) - 1; i >= 0; i-- {
			event := billingService.Status.Events[i]
			if event.ProductID != productID {
				continue
			}
			if event.State == string(BillingEventStateSent) {
				prunableIndices = append(prunableIndices, i)
			}
		}

		// Prune oldest events for this product
		pruneCount := toPrune
		if pruneCount > len(prunableIndices) {
			pruneCount = len(prunableIndices)
		}

		for i := 0; i < pruneCount; i++ {
			eventsToRemove[prunableIndices[i]] = true
			totalPruned++
		}
	}

	if totalPruned == 0 {
		return 0
	}

	// Build new events list without pruned events
	newEvents := make([]vshnv1.BillingEventStatus, 0, len(billingService.Status.Events)-totalPruned)
	for i, event := range billingService.Status.Events {
		if !eventsToRemove[i] {
			newEvents = append(newEvents, event)
		}
	}

	billingService.Status.Events = newEvents
	return totalPruned
}

// findEvent returns the oldest event matching the given findEventOpts.
// It returns the event index, the event itself, and true if found. Otherwise it returns -1, empty, and false.
func findEvent(billingService *vshnv1.BillingService, opts findEventOpts) (int, vshnv1.BillingEventStatus, bool) {
	events := billingService.Status.Events
	if len(events) == 0 {
		return -1, vshnv1.BillingEventStatus{}, false
	}

	stateSet := map[string]struct{}{}
	for _, s := range opts.States {
		stateSet[string(s)] = struct{}{}
	}

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if len(stateSet) > 0 {
			if _, ok := stateSet[event.State]; !ok {
				continue
			}
		}
		if opts.Type != nil && event.Type != string(*opts.Type) {
			continue
		}
		if opts.ProductID != nil && event.ProductID != *opts.ProductID {
			continue
		}
		return i, event, true
	}
	return -1, vshnv1.BillingEventStatus{}, false
}

// hasSentEvent returns true if there is a sent event of the given type optionally filtered by productID and value.
func hasSentEvent(billingService *vshnv1.BillingService, eventType BillingEventType, productID string, value string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type != string(eventType) || event.State != string(BillingEventStateSent) {
			continue
		}
		if productID != "" && event.ProductID != productID {
			continue
		}
		if value != "" && event.Value != value {
			continue
		}
		return true
	}
	return false
}

// hasEvent returns true if an event exists for the given productID.
func hasEvent(billingService *vshnv1.BillingService, eventType BillingEventType, productID string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(eventType) && event.ProductID == productID {
			return true
		}
	}
	return false
}

// hasEventWithValue returns true if an event of type t exists for (productID,value) (any state).
func hasEventWithValue(billingService *vshnv1.BillingService, eventType BillingEventType, productID, value string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(eventType) && event.ProductID == productID && event.Value == value {
			return true
		}
	}
	return false
}

// hasOpenCreated returns true if there is a created for productID that is not superseded.
func hasOpenCreated(billingService *vshnv1.BillingService, productID string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeCreated) && event.ProductID == productID && event.State != string(BillingEventStateSuperseded) {
			return true
		}
	}
	return false
}

// lastActiveSentProduct returns the most recent sent created event that has not been superseded.
func lastActiveSentProduct(billingService *vshnv1.BillingService) (productID string, value string, ok bool) {
	deleted := map[string]struct{}{}

	for _, event := range billingService.Status.Events {
		if event.State != string(BillingEventStateSent) {
			continue
		}
		if event.Type == string(BillingEventTypeDeleted) {
			deleted[event.ProductID] = struct{}{}
			continue
		}
		if event.Type == string(BillingEventTypeCreated) {
			if _, seen := deleted[event.ProductID]; !seen {
				return event.ProductID, event.Value, true
			}
		}
	}
	return "", "", false
}

// supersedeCreatedForSLA marks obsolete created events as superseded.
func supersedeCreatedForSLA(billingService *vshnv1.BillingService, currentProductID string) bool {
	changed := false
	keptCurrent := false

	for i := range billingService.Status.Events {
		event := billingService.Status.Events[i]
		if event.Type != string(BillingEventTypeCreated) {
			continue
		}
		if event.State == string(BillingEventStateSent) || event.State == string(BillingEventStateSuperseded) {
			continue
		}

		if event.ProductID != currentProductID {
			event.State = string(BillingEventStateSuperseded)
			billingService.Status.Events[i] = event
			changed = true
			continue
		}

		// current product, keep only the newest open created
		if keptCurrent {
			event.State = string(BillingEventStateSuperseded)
			billingService.Status.Events[i] = event
			changed = true
			continue
		}
		keptCurrent = true
	}
	return changed
}

// lastObservedValueForProduct returns last sent value for productID or fallback if none found.
func lastObservedValueForProduct(billingService *vshnv1.BillingService, productID string, fallback string) string {
	if value, ok := lastSentValueForProduct(billingService, productID); ok {
		return value
	}
	return fallback
}

// lastSentValueForProduct returns the value from the most recent SENT created/scaled for productID.
func lastSentValueForProduct(billingService *vshnv1.BillingService, productID string) (string, bool) {
	for _, event := range billingService.Status.Events {
		if event.State != string(BillingEventStateSent) {
			continue
		}
		if event.ProductID != productID {
			continue
		}
		if event.Type == string(BillingEventTypeScaled) || event.Type == string(BillingEventTypeCreated) {
			return event.Value, true
		}
	}
	return "", false
}

// hasBacklog returns true if there are any events not in sent or superseded.
func hasBacklog(billingService *vshnv1.BillingService) bool {
	for _, event := range billingService.Status.Events {
		if event.State != string(BillingEventStateSent) && event.State != string(BillingEventStateSuperseded) {
			return true
		}
	}
	return false
}
