package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
							{ProductID: "prod-123", Value: "1", Unit: "instance"},
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
							{ProductID: "prod-123", Value: "1", Unit: "instance"},
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
							{ProductID: "prod-123", Value: "1", Unit: "instance"},
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
							{ProductID: "prod-123", Value: "1", Unit: "instance"},
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
							{ProductID: "prod-123", Value: "1", Unit: "instance"},
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
