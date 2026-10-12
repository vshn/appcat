package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHandleItemCreation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	tests := []struct {
		name            string
		billingService  *vshnv1.BillingService
		item            vshnv1.ItemSpec
		initialCreation bool
		expectEvent     bool
		expectedType    string
		// expectCreationTS: event timestamp must equal the instance creation annotation,
		// otherwise it must be close to now
		expectCreationTS bool
	}{
		{
			name: "creates event for new item",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
					Annotations: map[string]string{
						vshnv1.InstanceCreationTimestampAnnotation: "2024-01-01T00:00:00Z",
					},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						ServiceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				Value:                "2",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			initialCreation:  true,
			expectEvent:      true,
			expectedType:     string(BillingEventTypeCreated),
			expectCreationTS: true,
		},
		{
			name: "does not create duplicate event for existing item",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						ServiceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", InstanceID: "test-service-inst-123", Value: "2", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:       string(BillingEventTypeCreated),
							ProductID:  "prod-123",
							InstanceID: "test-service-inst-123",
							Value:      "2",
							State:      string(BillingEventStatePending),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				InstanceID:           "test-service-inst-123",
				Value:                "2",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			expectEvent: false,
		},
		{
			name: "creates event for second item",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
					Annotations: map[string]string{
						vshnv1.InstanceCreationTimestampAnnotation: "2024-01-01T00:00:00Z",
					},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						ServiceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", InstanceID: "test-service-inst-123", Value: "2", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
							{ProductID: "prod-456", InstanceID: "test-service-inst-456", Value: "50Gi", ItemDescription: "Storage Item", ItemGroupDescription: "Storage Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:       string(BillingEventTypeCreated),
							ProductID:  "prod-123",
							InstanceID: "test-service-inst-123",
							Value:      "2",
							State:      string(BillingEventStateSent),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-456",
				InstanceID:           "test-service-inst-456",
				Value:                "50Gi",
				ItemDescription:      "Storage Item",
				ItemGroupDescription: "Storage Group",
			},
			initialCreation:  false,
			expectEvent:      true,
			expectedType:     string(BillingEventTypeCreated),
			expectCreationTS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.billingService).
				WithStatusSubresource(tt.billingService).
				Build()

			handler := &BillingHandler{
				Client:    client,
				Scheme:    scheme,
				maxEvents: 100,
			}

			initialEventCount := len(tt.billingService.Status.Events)
			err := handler.handleItemCreation(context.Background(), tt.billingService, tt.item, tt.initialCreation)
			require.NoError(t, err)

			if tt.expectEvent {
				assert.Equal(t, initialEventCount+1, len(tt.billingService.Status.Events),
					"expected new event to be added")
				newEvent := tt.billingService.Status.Events[0]
				assert.Equal(t, tt.expectedType, newEvent.Type)
				assert.Equal(t, tt.item.ProductID, newEvent.ProductID)
				assert.Equal(t, tt.item.Value, newEvent.Value)
				assert.Equal(t, tt.item.ItemDescription, newEvent.ItemDescription)
				assert.Equal(t, tt.item.ItemGroupDescription, newEvent.ItemGroupDescription)
				assert.Equal(t, string(BillingEventStatePending), newEvent.State)
				if tt.expectCreationTS {
					assert.Equal(t, "2024-01-01T00:00:00Z", newEvent.Timestamp.UTC().Format(time.RFC3339))
				} else {
					assert.WithinDuration(t, time.Now(), newEvent.Timestamp.Time, time.Minute)
				}
			} else {
				assert.Equal(t, initialEventCount, len(tt.billingService.Status.Events),
					"expected no new event to be added")
			}
		})
	}
}

func TestHandleItemCreation_MultipleItems(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-ns",
			Annotations: map[string]string{
				vshnv1.InstanceCreationTimestampAnnotation: "2024-01-01T00:00:00Z",
			},
		},
		Spec: vshnv1.BillingServiceSpec{
			Odoo: vshnv1.OdooSpec{
				ServiceID: "test-instance",
				Items: []vshnv1.ItemSpec{
					{ProductID: "prod-compute", InstanceID: "test-service-inst-compute", Value: "2", ItemDescription: "Compute Item", ItemGroupDescription: "Compute Group"},
					{ProductID: "prod-storage", InstanceID: "test-service-inst-storage", Value: "50Gi", ItemDescription: "Storage Item", ItemGroupDescription: "Storage Group"},
					{ProductID: "prod-backup", InstanceID: "test-service-inst-backup", Value: "enabled", ItemDescription: "Backup Item", ItemGroupDescription: "Backup Group"},
				},
			},
		},
		Status: vshnv1.BillingServiceStatus{
			Events: []vshnv1.BillingEventStatus{},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(billingService).
		WithStatusSubresource(billingService).
		Build()

	handler := &BillingHandler{
		Client:    client,
		Scheme:    scheme,
		maxEvents: 100,
	}

	// Create events for all three items
	for _, item := range billingService.Spec.Odoo.Items {
		err := handler.handleItemCreation(context.Background(), billingService, item, true)
		require.NoError(t, err)
	}

	// Verify all three items have created events
	assert.Equal(t, 3, len(billingService.Status.Events))

	productIDs := make(map[string]bool)
	for _, event := range billingService.Status.Events {
		assert.Equal(t, string(BillingEventTypeCreated), event.Type)
		assert.Equal(t, string(BillingEventStatePending), event.State)
		// All initial items must use the instance creation timestamp, not only the first one
		assert.Equal(t, "2024-01-01T00:00:00Z", event.Timestamp.UTC().Format(time.RFC3339))
		productIDs[event.ProductID] = true
	}

	assert.True(t, productIDs["prod-compute"])
	assert.True(t, productIDs["prod-storage"])
	assert.True(t, productIDs["prod-backup"])
}

// TestHandleItemCreation_PlanChange covers an item added to an existing BillingService
// (plan change: old compute item removed, new compute item added). Both the delete and the
// create event must use the time of the change, not the instance creation timestamp.
func TestHandleItemCreation_PlanChange(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	creationTS := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-ns",
			Annotations: map[string]string{
				vshnv1.InstanceCreationTimestampAnnotation: "2024-01-01T00:00:00Z",
			},
		},
		Spec: vshnv1.BillingServiceSpec{
			Odoo: vshnv1.OdooSpec{
				ServiceID: "test-instance",
				Items: []vshnv1.ItemSpec{
					{ProductID: "prod-compute-small", InstanceID: "test-service-inst-compute-small", Value: "1", ItemDescription: "Compute Small", ItemGroupDescription: "Compute Group"},
					{ProductID: "prod-storage", InstanceID: "test-service-inst-storage", Value: "50Gi", ItemDescription: "Storage Item", ItemGroupDescription: "Storage Group"},
				},
			},
		},
		Status: vshnv1.BillingServiceStatus{
			Events: []vshnv1.BillingEventStatus{
				{Type: string(BillingEventTypeCreated), ProductID: "prod-storage", InstanceID: "test-service-inst-storage", Value: "50Gi", Timestamp: creationTS, State: string(BillingEventStateSent)},
				{Type: string(BillingEventTypeCreated), ProductID: "prod-compute-mini", InstanceID: "test-service-inst-compute-mini", Value: "1", Timestamp: creationTS, State: string(BillingEventStateSent)},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(billingService).
		WithStatusSubresource(billingService).
		Build()

	handler := &BillingHandler{
		Client:    client,
		Scheme:    scheme,
		maxEvents: 100,
	}

	// Same order as Reconcile: creation per item, then removed items
	initialCreation := len(billingService.Status.Events) == 0
	for _, item := range billingService.Spec.Odoo.Items {
		require.NoError(t, handler.handleItemCreation(context.Background(), billingService, item, initialCreation))
	}
	require.NoError(t, handler.handleRemovedItems(context.Background(), billingService))

	require.Equal(t, 4, len(billingService.Status.Events))

	_, createEvent, found := findEventByInstanceID(billingService.Status.Events, BillingEventTypeCreated, "test-service-inst-compute-small")
	require.True(t, found, "expected create event for new compute item")
	assert.WithinDuration(t, time.Now(), createEvent.Timestamp.Time, time.Minute,
		"create event of an item added later must not use the instance creation timestamp")

	_, deleteEvent, found := findEventByInstanceID(billingService.Status.Events, BillingEventTypeDeleted, "test-service-inst-compute-mini")
	require.True(t, found, "expected delete event for old compute item")
	assert.WithinDuration(t, time.Now(), deleteEvent.Timestamp.Time, time.Minute)
}

// TestHandleItemCreation_AfterPruning covers a BillingService whose create events were pruned:
// a new item must still be treated as added later.
func TestHandleItemCreation_AfterPruning(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-ns",
			Annotations: map[string]string{
				vshnv1.InstanceCreationTimestampAnnotation: "2024-01-01T00:00:00Z",
			},
		},
		Status: vshnv1.BillingServiceStatus{
			Events: []vshnv1.BillingEventStatus{
				{Type: string(BillingEventTypeScaled), ProductID: "prod-storage", InstanceID: "test-service-inst-storage", Value: "100Gi", State: string(BillingEventStateSent)},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(billingService).
		WithStatusSubresource(billingService).
		Build()

	handler := &BillingHandler{
		Client:    client,
		Scheme:    scheme,
		maxEvents: 100,
	}

	item := vshnv1.ItemSpec{ProductID: "prod-backup", InstanceID: "test-service-inst-backup", Value: "enabled"}
	initialCreation := len(billingService.Status.Events) == 0
	require.NoError(t, handler.handleItemCreation(context.Background(), billingService, item, initialCreation))

	require.Equal(t, 2, len(billingService.Status.Events))
	newEvent := billingService.Status.Events[0]
	assert.Equal(t, string(BillingEventTypeCreated), newEvent.Type)
	assert.WithinDuration(t, time.Now(), newEvent.Timestamp.Time, time.Minute)
}

func TestHandleItemCreation_MissingAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	item := vshnv1.ItemSpec{ProductID: "prod-123", InstanceID: "test-service-inst-123", Value: "2"}

	tests := []struct {
		name            string
		initialCreation bool
		expectErr       bool
	}{
		{name: "initial creation requeues without annotation", initialCreation: true, expectErr: true},
		{name: "item added later does not need annotation", initialCreation: false, expectErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			billingService := &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(billingService).
				WithStatusSubresource(billingService).
				Build()

			handler := &BillingHandler{
				Client:    client,
				Scheme:    scheme,
				maxEvents: 100,
			}

			err := handler.handleItemCreation(context.Background(), billingService, item, tt.initialCreation)
			if tt.expectErr {
				require.Error(t, err)
				assert.Empty(t, billingService.Status.Events)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(billingService.Status.Events))
			}
		})
	}
}

func findEventByInstanceID(events []vshnv1.BillingEventStatus, eventType BillingEventType, instanceID string) (int, vshnv1.BillingEventStatus, bool) {
	for i, e := range events {
		if e.Type == string(eventType) && e.InstanceID == instanceID {
			return i, e, true
		}
	}
	return -1, vshnv1.BillingEventStatus{}, false
}
