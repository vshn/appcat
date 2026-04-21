package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

type findEventOpts struct {
	States []BillingEventState
	Type   *BillingEventType
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
	// Count events per instance
	eventCountPerInstance := make(map[string]int)
	for _, event := range billingService.Status.Events {
		eventCountPerInstance[event.InstanceID]++
	}

	// Build pruning list per instance
	eventsToRemove := make(map[int]bool)
	totalPruned := 0

	// Prune events for ALL instances (including removed ones)
	// This prevents unbounded growth of events for instances that were removed from spec
	for instanceID := range eventCountPerInstance {
		currentCount := eventCountPerInstance[instanceID]
		if currentCount <= maxEvents {
			continue // no pruning needed for this instance
		}

		toPrune := currentCount - maxEvents

		// Find oldest sent events for this instance — only sent events are prunable
		prunableIndices := []int{}
		for i := len(billingService.Status.Events) - 1; i >= 0; i-- {
			event := billingService.Status.Events[i]
			if event.InstanceID != instanceID {
				continue
			}
			if event.State == string(BillingEventStateSent) {
				prunableIndices = append(prunableIndices, i)
			}
		}

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
		return i, event, true
	}
	return -1, vshnv1.BillingEventStatus{}, false
}

// hasEventByInstanceID returns true if an event of the given type exists for the given instanceID.
func hasEventByInstanceID(billingService *vshnv1.BillingService, eventType BillingEventType, instanceID string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(eventType) && event.InstanceID == instanceID {
			return true
		}
	}
	return false
}

// hasEventWithValueByInstanceID returns true if an event of type t exists for (instanceID,value) (any state).
func hasEventWithValueByInstanceID(billingService *vshnv1.BillingService, eventType BillingEventType, instanceID, value string) bool {
	for _, event := range billingService.Status.Events {
		if event.Type == string(eventType) && event.InstanceID == instanceID && event.Value == value {
			return true
		}
	}
	return false
}

// lastSentValueForInstanceID returns the value from the most recent SENT created/scaled for instanceID.
func lastSentValueForInstanceID(billingService *vshnv1.BillingService, instanceID string) (string, bool) {
	for _, event := range billingService.Status.Events {
		if event.State != string(BillingEventStateSent) {
			continue
		}
		if event.InstanceID != instanceID {
			continue
		}
		if event.Type == string(BillingEventTypeScaled) || event.Type == string(BillingEventTypeCreated) {
			return event.Value, true
		}
	}
	return "", false
}

// lastObservedValueForInstanceID returns last sent value for instanceID or fallback if none found.
func lastObservedValueForInstanceID(billingService *vshnv1.BillingService, instanceID string, fallback string) string {
	if value, ok := lastSentValueForInstanceID(billingService, instanceID); ok {
		return value
	}
	return fallback
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
