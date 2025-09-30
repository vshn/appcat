package billing

import vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

// handleResendAnnotation goes through all events and resends them according to mode defined by the appcat.vshn.io/resend annotation.
func handleResendAnnotation(billingService *vshnv1.BillingService, mode string) bool {
	changed := false
	switch mode {
	case ResendAll:
		for i := range billingService.Status.Events {
			if billingService.Status.Events[i].State == string(BillingEventStateSuperseded) {
				continue
			}
			if billingService.Status.Events[i].State != string(BillingEventStateFailed) {
				billingService.Status.Events[i].State = string(BillingEventStateFailed)
				billingService.Status.Events[i].RetryCount++
				changed = true
			}
		}
	case ResendNotSent:
		for i := range billingService.Status.Events {
			if billingService.Status.Events[i].State == string(BillingEventStateSuperseded) {
				continue
			}
			if billingService.Status.Events[i].State != string(BillingEventStateSent) {
				if billingService.Status.Events[i].State != string(BillingEventStateFailed) {
					billingService.Status.Events[i].RetryCount++
				}
				billingService.Status.Events[i].State = string(BillingEventStateFailed)
				changed = true
			}
		}
	case ResendFailed:
		for i := range billingService.Status.Events {
			if billingService.Status.Events[i].State == string(BillingEventStateFailed) {
				billingService.Status.Events[i].RetryCount++
				changed = true
			}
		}
	}
	return changed
}
