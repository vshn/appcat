package billing

import (
	"context"
	"time"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// handleItemCreation checks if each item/product has a created event
func (b *BillingHandler) handleItemCreation(ctx context.Context, billingService *vshnv1.BillingService, item vshnv1.ItemSpec) error {
	if hasEvent(billingService, BillingEventTypeCreated, item.ProductID) {
		return nil
	}

	event := vshnv1.BillingEventStatus{
		Type:                 string(BillingEventTypeCreated),
		ProductID:            item.ProductID,
		InstanceID:           item.InstanceID,
		Value:                item.Value,
		ItemDescription:      item.ItemDescription,
		ItemGroupDescription: item.ItemGroupDescription,
		Timestamp:            instanceCreationTimestamp(billingService),
		State:                string(BillingEventStatePending),
		RetryCount:           0,
	}

	return enqueueEvent(ctx, b, billingService, event)
}

func instanceCreationTimestamp(svc *vshnv1.BillingService) metav1.Time {
	if raw := svc.Annotations[InstanceCreationTimestampAnnotation]; raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil && !t.IsZero() {
			return metav1.Time{Time: t}
		}
	}
	return svc.ObjectMeta.CreationTimestamp
}
