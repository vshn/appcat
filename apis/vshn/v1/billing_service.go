package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// BillingServiceFinalizer is the finalizer used to protect BillingService resources from deletion
	BillingServiceFinalizer = "billing.appcat.vshn.io/delete-protection"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:object:generate=true
// +kubebuilder:resource:scope=Namespaced,categories=appcatbilling
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// BillingService represents a service instance for billing purposes
type BillingService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a BillingService
	Spec BillingServiceSpec `json:"spec"`

	// Status reflects the observed state of a BillingService
	Status BillingServiceStatus `json:"status,omitempty"`
}

// BillingServiceSpec defines the desired state of a BillingService
type BillingServiceSpec struct {
	// KeepAfterDeletion defines how many days to keep billing records after service deletion
	KeepAfterDeletion int `json:"keepAfterDeletion,omitempty"`

	// Odoo contains Odoo-specific billing configuration
	Odoo OdooSpec `json:"odoo,omitempty"`
}

// ItemSpec defines a single billable product/item
type ItemSpec struct {
	// ProductID identifies the product in the billing system
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	ProductID string `json:"productID"`

	// ItemDescription is a human-readable description of the billing item
	ItemDescription string `json:"itemDescription,omitempty"`

	// ItemGroupDescription describes the billing item group
	ItemGroupDescription string `json:"itemGroupDescription,omitempty"`

	// Unit defines the billing unit type for this product
	Unit string `json:"unit,omitempty"`

	// Value represents the billable metric for this product
	// Can be: replica count, disk size (e.g., "50Gi"), percentage, etc.
	Value string `json:"value"`
}

// OdooSpec defines Odoo-specific billing configuration
type OdooSpec struct {
	// InstanceID uniquely identifies the service instance in Odoo
	InstanceID string `json:"instanceID"`

	// SalesOrderID identifies the sales order in Odoo
	SalesOrderID string `json:"salesOrderID,omitempty"`

	// Organization used to identify sales order
	Organization string `json:"organization,omitempty"`

	// Items defines list of billable products for this instance
	// Each item represents a product with independent lifecycle and event tracking
	// +kubebuilder:validation:MinItems=1
	Items []ItemSpec `json:"items"`
}

// BillingServiceStatus defines the observed state of a BillingService
type BillingServiceStatus struct {
	// Events contains the history of billing events
	Events []BillingEventStatus `json:"events,omitempty"`

	// Conditions represent the latest available observations of the billing service's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BillingEventStatus represents the status of a billing event
type BillingEventStatus struct {
	// Type is the type of billing event (created, deleted, scaled)
	// +kubebuilder:validation:Enum="created";"deleted";"scaled"
	Type string `json:"type"`

	// ProductID identifies the product in the billing system
	ProductID string `json:"productId"`

	// Value represents the billable metric at the time of the event
	// Generic field supporting replica count, disk size, percentages, etc.
	Value string `json:"value"`

	// Timestamp when the event occurred
	Timestamp metav1.Time `json:"timestamp"`

	// State represents the current state of the event (sent, pending, failed, superseded)
	// +kubebuilder:validation:Enum="sent";"pending";"failed";"superseded";"resend"
	State string `json:"state"`

	// RetryCount tracks the number of retry attempts for failed events
	// +kubebuilder:default=0
	RetryCount int `json:"retryCount,omitempty"`

	// LastAttemptTime is when we last tried to send this event
	LastAttemptTime metav1.Time `json:"lastAttemptTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true

// BillingServiceList contains a list of BillingService
type BillingServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BillingService `json:"items"`
}
