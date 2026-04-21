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

func TestHandleDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, vshnv1.AddToScheme(scheme))

	now := metav1.Now()

	tests := []struct {
		name                  string
		billingService        *vshnv1.BillingService
		expectedDeleteCount   int
		expectedInstanceIDs   []string          // instanceIDs that must have a delete event
		unexpectedInstanceIDs []string          // instanceIDs that must NOT have a delete event
		expectedDeleteValues  map[string]string // instanceID → expected delete event Value
	}{
		{
			name: "creates delete events for all spec items",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1", ItemDescription: "A", ItemGroupDescription: "G"},
							{ProductID: "prod-b", InstanceID: "inst-b", Value: "2", ItemDescription: "B", ItemGroupDescription: "G"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-b", ProductID: "prod-b", Value: "2", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount: 2,
			expectedInstanceIDs: []string{"inst-a", "inst-b"},
		},
		{
			name: "catches item absent from spec but present in sent creates (transient spec gap)",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						// Only 1 item in spec — inst-b temporarily absent
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1", ItemDescription: "A", ItemGroupDescription: "G"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						// Both were sent to Odoo
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-b", ProductID: "prod-b", Value: "2", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount: 2,
			expectedInstanceIDs: []string{"inst-a", "inst-b"},
		},
		{
			name: "does not delete item with only pending create (never reached Odoo)",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1", ItemDescription: "A", ItemGroupDescription: "G"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSent)},
						// inst-b create is pending — never reached Odoo
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-b", ProductID: "prod-b", Value: "2", State: string(BillingEventStatePending)},
					},
				},
			},
			expectedDeleteCount:   1,
			expectedInstanceIDs:   []string{"inst-a"},
			unexpectedInstanceIDs: []string{"inst-b"},
		},
		{
			name: "does not create duplicate delete event",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1", ItemDescription: "A", ItemGroupDescription: "G"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						// Delete already enqueued (previous reconcile)
						{Type: string(BillingEventTypeDeleted), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStatePending)},
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount: 1, // still 1 — no duplicate
		},
		{
			name: "does not delete item with superseded create",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{Items: []vshnv1.ItemSpec{}},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSuperseded)},
					},
				},
			},
			expectedDeleteCount:   0,
			unexpectedInstanceIDs: []string{"inst-a"},
		},
		{
			name: "uses last scaled value for delete",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1", ItemDescription: "A", ItemGroupDescription: "G"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeScaled), InstanceID: "inst-a", ProductID: "prod-a", Value: "5", State: string(BillingEventStateSent)},
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", ProductID: "prod-a", Value: "1", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount:  1,
			expectedInstanceIDs:  []string{"inst-a"},
			expectedDeleteValues: map[string]string{"inst-a": "5"},
		},
		{
			name: "catches instance with only scaled event (create was pruned)",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-svc",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{vshnv1.BillingServiceFinalizer},
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{Items: []vshnv1.ItemSpec{}},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						// create event was pruned; only scale event remains
						{Type: string(BillingEventTypeScaled), InstanceID: "inst-a", ProductID: "prod-a", Value: "5", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount:  1,
			expectedInstanceIDs:  []string{"inst-a"},
			expectedDeleteValues: map[string]string{"inst-a": "5"},
		},
		{
			name: "no-op when DeletionTimestamp not set",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-a", InstanceID: "inst-a", Value: "1"},
						},
					},
				},
				Status: vshnv1.BillingServiceStatus{
					Events: []vshnv1.BillingEventStatus{
						{Type: string(BillingEventTypeCreated), InstanceID: "inst-a", State: string(BillingEventStateSent)},
					},
				},
			},
			expectedDeleteCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.billingService).
				WithStatusSubresource(tt.billingService).
				Build()

			handler := &BillingHandler{Client: client, Scheme: scheme, maxEvents: 100}

			err := handler.handleDeletion(context.Background(), tt.billingService)
			require.NoError(t, err)

			deleteCount := 0
			for _, e := range tt.billingService.Status.Events {
				if e.Type == string(BillingEventTypeDeleted) {
					deleteCount++
					assert.Equal(t, string(BillingEventStatePending), e.State)
					assert.NotEmpty(t, e.InstanceID)
					assert.NotEmpty(t, e.ProductID)
				}
			}
			assert.Equal(t, tt.expectedDeleteCount, deleteCount)

			for _, id := range tt.expectedInstanceIDs {
				assert.True(t, hasEventByInstanceID(tt.billingService, BillingEventTypeDeleted, id),
					"expected delete event for instanceID %s", id)
			}
			for _, id := range tt.unexpectedInstanceIDs {
				assert.False(t, hasEventByInstanceID(tt.billingService, BillingEventTypeDeleted, id),
					"unexpected delete event for instanceID %s", id)
			}
			for _, e := range tt.billingService.Status.Events {
				if e.Type != string(BillingEventTypeDeleted) {
					continue
				}
				if want, ok := tt.expectedDeleteValues[e.InstanceID]; ok {
					assert.Equal(t, want, e.Value, "delete value mismatch for instanceID %s", e.InstanceID)
				}
			}
		})
	}
}

func TestShouldRemoveFinalizer(t *testing.T) {
	now := time.Now()
	deletionTimestamp := metav1.NewTime(now)

	tests := []struct {
		name              string
		billingService    *vshnv1.BillingService
		expectedCanRemove bool
		expectedRequeue   bool
	}{
		{
			name: "KeepAfterDeletion is 0 - should remove immediately",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: vshnv1.BillingServiceSpec{
					KeepAfterDeletion: 0,
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "1", ItemDescription: "test", ItemGroupDescription: "test-group"},
						},
					},
				},
			},
			expectedCanRemove: true,
			expectedRequeue:   false,
		},
		{
			name: "within grace period - should not remove but requeue",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: vshnv1.BillingServiceSpec{
					KeepAfterDeletion: 7,
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "1", ItemDescription: "test", ItemGroupDescription: "test-group"},
						},
					},
				},
			},
			expectedCanRemove: false,
			expectedRequeue:   true,
		},
		{
			name: "grace period elapsed - should remove",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: func() *metav1.Time {
						past := metav1.NewTime(now.Add(-8 * 24 * time.Hour))
						return &past
					}(),
				},
				Spec: vshnv1.BillingServiceSpec{
					KeepAfterDeletion: 7,
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "1", ItemDescription: "test", ItemGroupDescription: "test-group"},
						},
					},
				},
			},
			expectedCanRemove: true,
			expectedRequeue:   false,
		},
		{
			name: "no KeepAfterDeletion set - should remove immediately",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: vshnv1.BillingServiceSpec{
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "1", ItemDescription: "test", ItemGroupDescription: "test-group"},
						},
					},
				},
			},
			expectedCanRemove: true,
			expectedRequeue:   false,
		},
		{
			name: "negative KeepAfterDeletion - should remove immediately",
			billingService: &vshnv1.BillingService{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: vshnv1.BillingServiceSpec{
					KeepAfterDeletion: -1,
					Odoo: vshnv1.OdooSpec{
						Items: []vshnv1.ItemSpec{
							{ProductID: "prod-123", Value: "1", ItemDescription: "test", ItemGroupDescription: "test-group"},
						},
					},
				},
			},
			expectedCanRemove: true,
			expectedRequeue:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canRemove, requeueAfter := shouldRemoveFinalizer(tt.billingService)
			assert.Equal(t, tt.expectedCanRemove, canRemove, "canRemove mismatch")

			if tt.expectedRequeue {
				assert.NotNil(t, requeueAfter, "expected requeue but got nil")
				assert.Greater(t, *requeueAfter, time.Duration(0), "requeue duration should be positive")
			} else {
				if !canRemove {
					assert.Nil(t, requeueAfter, "expected no requeue but got one")
				}
			}
		})
	}
}
