package billing

import (
	"context"
	"fmt"
	"time"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleItemCreation checks if each item/product has a created event
func (b *BillingHandler) handleItemCreation(ctx context.Context, billingService *vshnv1.BillingService, item vshnv1.ItemSpec) error {
	if hasEvent(billingService, BillingEventTypeCreated, item.ProductID) {
		return nil
	}

	ts, ok := instanceCreationTimestamp(billingService)
	if !ok {
		// Annotation not yet set by the comp-function (race: provider-kubernetes may apply
		// the manifest in two reconcile steps, creating the object before the annotation lands).
		// Return an error so the controller requeues until the annotation is present.
		return fmt.Errorf("instance creation timestamp annotation %q not yet set on %s — requeueing",
			vshnv1.InstanceCreationTimestampAnnotation, billingService.Name)
	}

	event := vshnv1.BillingEventStatus{
		Type:                 string(BillingEventTypeCreated),
		ProductID:            item.ProductID,
		InstanceID:           item.InstanceID,
		Value:                item.Value,
		ItemDescription:      item.ItemDescription,
		ItemGroupDescription: item.ItemGroupDescription,
		Timestamp:            ts,
		State:                string(BillingEventStatePending),
		RetryCount:           0,
	}

	return enqueueEvent(ctx, b, billingService, event)
}

// instanceCreationTimestamp returns the creation timestamp of the originating composite/claim
// from the annotation set by the comp-function, plus true.
// Returns zero time and false if the annotation is absent or unparseable (caller should requeue).
func instanceCreationTimestamp(svc *vshnv1.BillingService) (metav1.Time, bool) {
	if raw := svc.Annotations[vshnv1.InstanceCreationTimestampAnnotation]; raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil && !t.IsZero() {
			return metav1.Time{Time: t}, true
		}
	}
	return metav1.Time{}, false
}
