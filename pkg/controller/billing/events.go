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

// enqueueEvent prepends a new billing event to the list and updates the BillingService status.
func enqueueEvent(ctx context.Context, b *BillingHandler, billingService *vshnv1.BillingService, event vshnv1.BillingEventStatus) error {
	billingService.Status.Events = append([]vshnv1.BillingEventStatus{event}, billingService.Status.Events...)
	return b.Status().Update(ctx, billingService)
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

// hasSentEvent returns true if there is a sent event of the given type optionally filtered by productID and size.
func hasSentEvent(billingService *vshnv1.BillingService, eventType BillingEventType, productID string, size string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type != string(eventType) || event.State != string(BillingEventStateSent) {
			continue
		}
		if productID != "" && event.ProductID != productID {
			continue
		}
		if size != "" && event.Size != size {
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

// hasEventWithSize returns true if an event of type t exists for (productID,size) (any state).
func hasEventWithSize(billingService *vshnv1.BillingService, eventType BillingEventType, productID, size string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(eventType) && event.ProductID == productID && event.Size == size {
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
func lastActiveSentProduct(billingService *vshnv1.BillingService) (productID string, size string, ok bool) {
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
				return event.ProductID, event.Size, true
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

// lastObservedSizeForProduct returns last sent size for productID or fallback if none found.
func lastObservedSizeForProduct(billingService *vshnv1.BillingService, productID string, fallback string) string {
	if size, ok := lastSentSizeForProduct(billingService, productID); ok {
		return size
	}
	return fallback
}

// lastSentSizeForProduct returns the size from the most recent SENT created/scaled for productID.
func lastSentSizeForProduct(billingService *vshnv1.BillingService, productID string) (string, bool) {
	for _, event := range billingService.Status.Events {
		if event.State != string(BillingEventStateSent) {
			continue
		}
		if event.ProductID != productID {
			continue
		}
		if event.Type == string(BillingEventTypeScaled) || event.Type == string(BillingEventTypeCreated) {
			return event.Size, true
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
