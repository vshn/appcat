package billing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHasEvent(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		eventType      BillingEventType
		productID      string
		expected       bool
	}{
		{
			name: "returns true when event exists",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "prod-123",
			expected:  true,
		},
		{
			name: "returns false when event does not exist",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeDeleted,
			productID: "prod-123",
			expected:  false,
		},
		{
			name: "returns false for different productID",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "prod-456",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasEvent(tt.billingService, tt.eventType, tt.productID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasSentEvent(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		eventType      BillingEventType
		productID      string
		value          string
		expected       bool
	}{
		{
			name: "returns true for sent event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "prod-123",
			value:     "2",
			expected:  true,
		},
		{
			name: "returns false for pending event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "prod-123",
			value:     "2",
			expected:  false,
		},
		{
			name: "returns false for different value",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "prod-123",
			value:     "3",
			expected:  false,
		},
		{
			name: "works with empty productID filter",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			eventType: BillingEventTypeCreated,
			productID: "",
			value:     "",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSentEvent(tt.billingService, tt.eventType, tt.productID, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasEventWithValue(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		eventType      BillingEventType
		productID      string
		value          string
		expected       bool
	}{
		{
			name: "returns true for matching event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							Value:     "5",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeScaled,
			productID: "prod-123",
			value:     "5",
			expected:  true,
		},
		{
			name: "returns false for different value",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							Value:     "5",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			eventType: BillingEventTypeScaled,
			productID: "prod-123",
			value:     "3",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasEventWithValue(tt.billingService, tt.eventType, tt.productID, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLastSentValueForProduct(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		productID      string
		expectedValue  string
		expectedOk     bool
	}{
		{
			name: "returns value from sent created event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			productID:     "prod-123",
			expectedValue: "2",
			expectedOk:    true,
		},
		{
			name: "returns value from most recent sent scaled event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							Value:     "5",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			productID:     "prod-123",
			expectedValue: "5",
			expectedOk:    true,
		},
		{
			name: "returns false when no sent event exists",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			productID:     "prod-123",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name: "returns false for non-existent product",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			productID:     "prod-456",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name: "ignores delete events",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeDeleted),
							ProductID: "prod-123",
							Value:     "5",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			productID:     "prod-123",
			expectedValue: "2",
			expectedOk:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := lastSentValueForProduct(tt.billingService, tt.productID)
			assert.Equal(t, tt.expectedOk, ok)
			if tt.expectedOk {
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestHasOpenCreated(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		productID      string
		expected       bool
	}{
		{
			name: "returns true for non-superseded created event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			productID: "prod-123",
			expected:  true,
		},
		{
			name: "returns false for superseded created event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							State:     string(BillingEventStateSuperseded),
						},
					},
				},
			},
			productID: "prod-123",
			expected:  false,
		},
		{
			name: "returns false when no created event exists",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			productID: "prod-123",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasOpenCreated(tt.billingService, tt.productID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasBacklog(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		expected       bool
	}{
		{
			name: "returns true for pending events",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:  string(BillingEventTypeCreated),
							State: string(BillingEventStatePending),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "returns true for failed events",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:  string(BillingEventTypeCreated),
							State: string(BillingEventStateFailed),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "returns false when all events are sent",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:  string(BillingEventTypeCreated),
							State: string(BillingEventStateSent),
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "returns false when all events are sent or superseded",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:  string(BillingEventTypeCreated),
							State: string(BillingEventStateSent),
						},
						{
							Type:  string(BillingEventTypeCreated),
							State: string(BillingEventStateSuperseded),
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "returns false for empty events",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasBacklog(tt.billingService)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPruneEventsIfNeeded(t *testing.T) {
	tests := []struct {
		name                string
		billingService      *vshnv1.BillingService
		maxEvents           int
		expectedPruned      int
		expectedRemaining   int
		checkRemainingState bool
	}{
		{
			name: "prunes oldest sent events when exceeding limit",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "5", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "4", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "3", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", Value: "2", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
					},
				},
			},
			maxEvents:         2,
			expectedPruned:    2,
			expectedRemaining: 2,
		},
		{
			name: "does not prune when under limit",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", Value: "2", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-456", Value: "50Gi", State: string(BillingEventStateSent)},
					},
				},
			},
			maxEvents:         5,
			expectedPruned:    0,
			expectedRemaining: 2,
		},
		{
			name: "does not prune pending events",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "5", State: string(BillingEventStatePending), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "4", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "3", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", Value: "2", State: string(BillingEventStateSent), Timestamp: metav1.Now()},
					},
				},
			},
			maxEvents:           2,
			expectedPruned:      2,
			expectedRemaining:   2,
			checkRemainingState: true,
		},
		{
			name: "prunes per-product independently",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", Value: "5", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", Value: "2", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeScaled), ProductID: "prod-456", Value: "100Gi", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-456", Value: "50Gi", State: string(BillingEventStateSent)},
					},
				},
			},
			maxEvents:         1,
			expectedPruned:    2,
			expectedRemaining: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruned := pruneEventsIfNeeded(tt.billingService, tt.maxEvents)
			assert.Equal(t, tt.expectedPruned, pruned)
			assert.Equal(t, tt.expectedRemaining, len(tt.billingService.Status.Events))

			if tt.checkRemainingState {
				// Verify pending event was not pruned
				hasPending := false
				for _, event := range tt.billingService.Status.Events {
					if event.State == string(BillingEventStatePending) {
						hasPending = true
						break
					}
				}
				assert.True(t, hasPending, "pending event should not be pruned")
			}
		})
	}
}

func TestFindEvent(t *testing.T) {
	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		opts           findEventOpts
		expectedFound  bool
		expectedIndex  int
		expectedType   string
	}{
		{
			name: "finds oldest pending event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", State: string(BillingEventStatePending)},
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", State: string(BillingEventStatePending)},
					},
				},
			},
			opts: findEventOpts{
				States: []BillingEventState{BillingEventStatePending},
			},
			expectedFound: true,
			expectedIndex: 1,
			expectedType:  string(BillingEventTypeCreated),
		},
		{
			name: "finds event by type",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), ProductID: "prod-123", State: string(BillingEventStatePending)},
						{Type: string(BillingEventTypeDeleted), ProductID: "prod-456", State: string(BillingEventStatePending)},
					},
				},
			},
			opts: findEventOpts{
				States: []BillingEventState{BillingEventStatePending},
				Type:   ptrTo(BillingEventTypeDeleted),
			},
			expectedFound: true,
			expectedIndex: 1,
			expectedType:  string(BillingEventTypeDeleted),
		},
		{
			name: "returns false when no matching event",
			billingService: &vshnv1.BillingService{
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), ProductID: "prod-123", State: string(BillingEventStateSent)},
					},
				},
			},
			opts: findEventOpts{
				States: []BillingEventState{BillingEventStatePending},
			},
			expectedFound: false,
			expectedIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, event, found := findEvent(tt.billingService, tt.opts)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedIndex, idx)
			if tt.expectedFound {
				assert.Equal(t, tt.expectedType, event.Type)
			}
		})
	}
}

// Helper function for tests
func ptrTo(t BillingEventType) *BillingEventType {
	return &t
}
