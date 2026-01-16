package billing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHandleRemovedItems(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	tests := []struct {
		name                 string
		billingService       *vshnv1.BillingService
		expectDeleteEvents   []string // productIDs that should have delete events
		expectNoDeleteEvents []string // productIDs that should not have delete events
	}{
		{
			name: "creates delete event for removed item",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-456",
							Value:     "50Gi",
							Unit:      "storage",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			expectDeleteEvents:   []string{"prod-456"},
			expectNoDeleteEvents: []string{"prod-123"},
		},
		{
			name: "does not create delete event when no items removed",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance"},
							{ProductID: "prod-456", Value: "50Gi", Unit: "storage"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-456",
							Value:     "50Gi",
							Unit:      "storage",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			expectDeleteEvents:   []string{},
			expectNoDeleteEvents: []string{"prod-123", "prod-456"},
		},
		{
			name: "creates delete events for multiple removed items",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-456",
							Value:     "50Gi",
							Unit:      "storage",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-789",
							Value:     "enabled",
							Unit:      "boolean",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			expectDeleteEvents:   []string{"prod-456", "prod-789"},
			expectNoDeleteEvents: []string{"prod-123"},
		},
		{
			name: "does not create duplicate delete event",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeDeleted),
							ProductID: "prod-456",
							Value:     "50Gi",
							State:     string(BillingEventStatePending),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-456",
							Value:     "50Gi",
							Unit:      "storage",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			expectDeleteEvents:   []string{},
			expectNoDeleteEvents: []string{"prod-123"},
		},
		{
			name: "ignores superseded created events",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-456",
							Value:     "50Gi",
							Unit:      "storage",
							State:     string(BillingEventStateSuperseded),
						},
					},
				},
			},
			expectDeleteEvents:   []string{},
			expectNoDeleteEvents: []string{"prod-123", "prod-456"},
		},
		{
			name: "uses last sent value for delete event",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items:      []vshnv1.ItemSpec{},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							Value:     "5",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "2",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			expectDeleteEvents: []string{"prod-123"},
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
			err := handler.handleRemovedItems(context.Background(), tt.billingService)
			require.NoError(t, err)

			// Check for expected delete events
			for _, productID := range tt.expectDeleteEvents {
				found := false
				for _, event := range tt.billingService.Status.Events {
					if event.Type == string(BillingEventTypeDeleted) && event.ProductID == productID {
						found = true
						assert.Equal(t, string(BillingEventStatePending), event.State)

						// Special check for "uses last sent value for delete event" test
						if tt.name == "uses last sent value for delete event" {
							assert.Equal(t, "5", event.Value, "should use last sent scaled value")
							assert.Equal(t, "instance", event.Unit, "should use last sent unit")
						}
						break
					}
				}
				assert.True(t, found, "expected delete event for product %s", productID)
			}

			// Check that no delete events exist for products that should not be deleted
			for _, productID := range tt.expectNoDeleteEvents {
				for _, event := range tt.billingService.Status.Events {
					if event.Type == string(BillingEventTypeDeleted) && event.ProductID == productID {
						t.Errorf("unexpected delete event for product %s", productID)
					}
				}
			}

			expectedEventCount := initialEventCount + len(tt.expectDeleteEvents)
			assert.Equal(t, expectedEventCount, len(tt.billingService.Status.Events))
		})
	}
}

func TestHandleRemovedItems_EmptySpec(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	billingService := &vshnv1.BillingService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-ns",
		},
		Spec: vshnv1.BillingServiceSpec{
			Odoo: vshnv1.OdooSpec{
				InstanceID: "test-instance",
				Items:      []vshnv1.ItemSpec{},
			},
		},
		Status: vshnv1.BillingServiceStatus{
			Events: []vshnv1.BillingEventStatus{
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-123",
					Value:     "2",
					Unit:      "instance",
					State:     string(BillingEventStateSent),
				},
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-456",
					Value:     "50Gi",
					Unit:      "storage",
					State:     string(BillingEventStateSent),
				},
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-789",
					Value:     "enabled",
					Unit:      "boolean",
					State:     string(BillingEventStateSent),
				},
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

	err := handler.handleRemovedItems(context.Background(), billingService)
	require.NoError(t, err)

	// Should create delete events for all three products
	deleteEvents := 0
	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeDeleted) {
			deleteEvents++
		}
	}
	assert.Equal(t, 3, deleteEvents)
	assert.Equal(t, 6, len(billingService.Status.Events)) // 3 created + 3 deleted
}
