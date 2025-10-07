package billing

import "time"

const (
	ResendAnnotationKey = "appcat.vshn.io/resend"
	ResendAll           = "all"
	ResendNotSent       = "not-sent"
	ResendFailed        = "failed"

	// Controller backoff for the rate limiter
	minBackoff = 10 * time.Second
	maxBackoff = 30 * time.Minute

	// Drain delay after a successful reconciliation
	successDrainDelay = 1 * time.Second
)

type BillingEventState string

const (
	BillingEventStatePending    BillingEventState = "pending"
	BillingEventStateSent       BillingEventState = "sent"
	BillingEventStateFailed     BillingEventState = "failed"
	BillingEventStateSuperseded BillingEventState = "superseded"
	BillingEventStateResend     BillingEventState = "resend"
)

type BillingEventType string

const (
	BillingEventTypeCreated BillingEventType = "created"
	BillingEventTypeDeleted BillingEventType = "deleted"
	BillingEventTypeScaled  BillingEventType = "scaled"
)

const (
	ConditionTypeSynced = "Synced"
	ConditionTypeReady  = "Ready"

	ReasonAllEventsSent    = "AllEventsSent"
	ReasonPendingOrFailed  = "PendingOrFailedEvents"
	ReasonReconcileSuccess = "ReconcileSuccess"
	ReasonDeleting         = "Deleting"
)
