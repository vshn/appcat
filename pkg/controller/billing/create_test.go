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

func TestHandleItemCreation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		item           vshnv1.ItemSpec
		expectEvent    bool
		expectedType   string
	}{
		{
			name: "creates event for new item",
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
					Events: []vshnv1.BillingEventStatus{},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID: "prod-123",
				Value:     "2",
				Unit:      "instance",
			},
			expectEvent:  true,
			expectedType: string(BillingEventTypeCreated),
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
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID: "prod-123",
				Value:     "2",
				Unit:      "instance",
			},
			expectEvent: false,
		},
		{
			name: "creates event for second item",
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
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID: "prod-456",
				Value:     "50Gi",
				Unit:      "storage",
			},
			expectEvent:  true,
			expectedType: string(BillingEventTypeCreated),
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
			err := handler.handleItemCreation(context.Background(), tt.billingService, tt.item)
			require.NoError(t, err)

			if tt.expectEvent {
				assert.Equal(t, initialEventCount+1, len(tt.billingService.Status.Events),
					"expected new event to be added")
				newEvent := tt.billingService.Status.Events[0]
				assert.Equal(t, tt.expectedType, newEvent.Type)
				assert.Equal(t, tt.item.ProductID, newEvent.ProductID)
				assert.Equal(t, tt.item.Value, newEvent.Value)
				assert.Equal(t, tt.item.Unit, newEvent.Unit)
				assert.Equal(t, string(BillingEventStatePending), newEvent.State)
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
		},
		Spec: vshnv1.BillingServiceSpec{
			Odoo: vshnv1.OdooSpec{
				InstanceID: "test-instance",
				Items: []vshnv1.ItemSpec{
					{ProductID: "prod-compute", Value: "2", Unit: "instance"},
					{ProductID: "prod-storage", Value: "50Gi", Unit: "storage"},
					{ProductID: "prod-backup", Value: "enabled", Unit: "boolean"},
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
		err := handler.handleItemCreation(context.Background(), billingService, item)
		require.NoError(t, err)
	}

	// Verify all three items have created events
	assert.Equal(t, 3, len(billingService.Status.Events))

	productIDs := make(map[string]bool)
	for _, event := range billingService.Status.Events {
		assert.Equal(t, string(BillingEventTypeCreated), event.Type)
		assert.Equal(t, string(BillingEventStatePending), event.State)
		productIDs[event.ProductID] = true
	}

	assert.True(t, productIDs["prod-compute"])
	assert.True(t, productIDs["prod-storage"])
	assert.True(t, productIDs["prod-backup"])
}
