package billing

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setSyncedCondition(billingService *vshnv1.BillingService) {
	allDeliveredOrIgnored := true
	for _, e := range billingService.Status.Events {
		if e.State != string(BillingEventStateSent) && e.State != string(BillingEventStateSuperseded) {
			allDeliveredOrIgnored = false
			break
		}
	}
	cond := metav1.Condition{
		Type:               ConditionTypeSynced,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonAllEventsSent,
		ObservedGeneration: billingService.Generation,
	}
	if !allDeliveredOrIgnored {
		cond.Status = metav1.ConditionFalse
		cond.Reason = ReasonPendingOrFailed
	}
	setCondition(&billingService.Status.Conditions, cond)
}

func setReadyCondition(billingService *vshnv1.BillingService) {
	ready := billingService.DeletionTimestamp.IsZero()
	cond := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonReconcileSuccess,
		ObservedGeneration: billingService.Generation,
	}
	if !ready {
		cond.Status = metav1.ConditionFalse
		cond.Reason = ReasonDeleting
	}
	setCondition(&billingService.Status.Conditions, cond)
}

func setCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	now := metav1.Now()
	condition.LastTransitionTime = now
	if i := findConditionIndex(*conditions, condition.Type); i >= 0 {
		prev := (*conditions)[i]
		if prev.Status == condition.Status && prev.Reason == condition.Reason {
			condition.LastTransitionTime = prev.LastTransitionTime
		}
		(*conditions)[i] = condition
		return
	}
	*conditions = append(*conditions, condition)
}

func findConditionIndex(conds []metav1.Condition, t string) int {
	for i := range conds {
		if conds[i].Type == t {
			return i
		}
	}
	return -1
}

func (b *BillingHandler) updateStatus(ctx context.Context, billingService *vshnv1.BillingService) error {
	setReadyCondition(billingService)
	setSyncedCondition(billingService)
	return b.Status().Update(ctx, billingService)
}
