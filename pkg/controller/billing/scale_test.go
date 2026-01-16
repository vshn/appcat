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

func TestHandleItemScaling(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	tests := []struct {
		name           string
		billingService *vshnv1.BillingService
		item           vshnv1.ItemSpec
		expectEvent    bool
		expectedValue  string
	}{
		{
			name: "creates scaled event when value changes",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "3", Unit: "instance", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "1",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				Value:                "3",
				Unit:                 "instance",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			expectEvent:   true,
			expectedValue: "3",
		},
		{
			name: "does not create scaled event when value unchanged",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "2", Unit: "instance", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
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
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				Value:                "2",
				Unit:                 "instance",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			expectEvent: false,
		},
		{
			name: "does not create scaled event when no previous sent event exists",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "3", Unit: "instance", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "1",
							State:     string(BillingEventStatePending),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				Value:                "3",
				Unit:                 "instance",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			expectEvent: false,
		},
		{
			name: "does not create duplicate scaled event",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "3", Unit: "instance", ItemDescription: "Test Item", ItemGroupDescription: "Test Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeScaled),
							ProductID: "prod-123",
							Value:     "3",
							State:     string(BillingEventStatePending),
						},
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-123",
							Value:     "1",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-123",
				Value:                "3",
				Unit:                 "instance",
				ItemDescription:      "Test Item",
				ItemGroupDescription: "Test Group",
			},
			expectEvent: false,
		},
		{
			name: "creates scaled event for storage size change",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						InstanceID: "test-instance",
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-storage", Value: "100Gi", Unit: "storage", ItemDescription: "Storage Item", ItemGroupDescription: "Storage Group"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{
							Type:      string(BillingEventTypeCreated),
							ProductID: "prod-storage",
							Value:     "50Gi",
							Unit:      "instance",
							State:     string(BillingEventStateSent),
						},
					},
				},
			},
			item: vshnv1.ItemSpec{
				ProductID:            "prod-storage",
				Value:                "100Gi",
				Unit:                 "storage",
				ItemDescription:      "Storage Item",
				ItemGroupDescription: "Storage Group",
			},
			expectEvent:   true,
			expectedValue: "100Gi",
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
			err := handler.handleItemScaling(context.Background(), tt.billingService, tt.item)
			require.NoError(t, err)

			if tt.expectEvent {
				assert.Equal(t, initialEventCount+1, len(tt.billingService.Status.Events),
					"expected new scaled event to be added")
				newEvent := tt.billingService.Status.Events[0]
				assert.Equal(t, string(BillingEventTypeScaled), newEvent.Type)
				assert.Equal(t, tt.item.ProductID, newEvent.ProductID)
				assert.Equal(t, tt.expectedValue, newEvent.Value)
				assert.Equal(t, tt.item.Unit, newEvent.Unit)
				assert.Equal(t, tt.item.ItemDescription, newEvent.ItemDescription)
				assert.Equal(t, tt.item.ItemGroupDescription, newEvent.ItemGroupDescription)
				assert.Equal(t, string(BillingEventStatePending), newEvent.State)
			} else {
				assert.Equal(t, initialEventCount, len(tt.billingService.Status.Events),
					"expected no new event to be added")
			}
		})
	}
}

func TestHandleItemScaling_MultipleItems(t *testing.T) {
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
					{ProductID: "prod-compute", Value: "4", Unit: "instance", ItemDescription: "Compute Item", ItemGroupDescription: "Compute Group"},
					{ProductID: "prod-storage", Value: "100Gi", Unit: "storage", ItemDescription: "Storage Item", ItemGroupDescription: "Storage Group"},
					{ProductID: "prod-backup", Value: "enabled", Unit: "boolean", ItemDescription: "Backup Item", ItemGroupDescription: "Backup Group"},
				},
			},
		},
		Status: vshnv1.BillingServiceStatus{
			Events: []vshnv1.BillingEventStatus{
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-compute",
					Value:     "2",
					Unit:      "instance",
					State:     string(BillingEventStateSent),
				},
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-storage",
					Value:     "50Gi",
					State:     string(BillingEventStateSent),
				},
				{
					Type:      string(BillingEventTypeCreated),
					ProductID: "prod-backup",
					Value:     "enabled",
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

	// Scale compute and storage, but not backup
	for _, item := range billingService.Spec.Odoo.Items {
		err := handler.handleItemScaling(context.Background(), billingService, item)
		require.NoError(t, err)
	}

	// Should have 5 events: 3 original created + 2 new scaled (compute and storage)
	assert.Equal(t, 5, len(billingService.Status.Events))

	// Verify scaled events were created for compute and storage only
	scaledEvents := 0
	for _, event := range billingService.Status.Events {
		if event.Type == string(BillingEventTypeScaled) {
			scaledEvents++
			assert.True(t, event.ProductID == "prod-compute" || event.ProductID == "prod-storage")
			if event.ProductID == "prod-compute" {
				assert.Equal(t, "4", event.Value)
			} else if event.ProductID == "prod-storage" {
				assert.Equal(t, "100Gi", event.Value)
			}
		}
	}
	assert.Equal(t, 2, scaledEvents)
}
