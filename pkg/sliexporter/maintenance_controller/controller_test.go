package maintenancecontroller

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	managedupgradev1beta1 "github.com/appuio/openshift-upgrade-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMaintenanceReconciler_isAnyJobRunning(t *testing.T) {
	tests := []struct {
		name             string
		initMaintenances []bool
		want             bool
		wantErr          bool
	}{
		{
			name: "GivenOneRunning_ThenExpectRunning",
			initMaintenances: []bool{
				true,
			},
			want: true,
		},
		{
			name: "GivenTwoRunning_ThenExpectRunning",
			initMaintenances: []bool{
				true,
				true,
			},
			want: true,
		},
		{
			name: "GivenOneOfTwoRunning_ThenExpectRunning",
			initMaintenances: []bool{
				false,
				true,
			},
			want: true,
		},
		{
			name: "GivenNoneRunning_ThenExpectNotRunning",
			initMaintenances: []bool{
				false,
				false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &MaintenanceReconciler{
				maintenance: atomic.Bool{},
				Client:      createFakeClient(tt.initMaintenances),
			}
			got, err := r.isAnyJobRunning(context.TODO(), logr.Discard())
			if (err != nil) != tt.wantErr {
				t.Errorf("MaintenanceReconciler.isAnyJobRunning() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MaintenanceReconciler.isAnyJobRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createFakeClient(maintenances []bool) client.Client {
	list := []client.Object{}
	for i, maintenance := range maintenances {
		cond := metav1.ConditionFalse
		if maintenance {
			cond = metav1.ConditionTrue
		}

		list = append(list, &managedupgradev1beta1.UpgradeJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("maintenance-%d", i),
				Namespace: "default",
			},
			Status: managedupgradev1beta1.UpgradeJobStatus{
				Conditions: []metav1.Condition{
					{
						LastTransitionTime: metav1.Time{Time: time.Now()},
						Message:            "Upgrade started",
						Reason:             "Started",
						Status:             cond,
						Type:               "Started",
					},
				},
			},
		})
	}

	return fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(list...).
		Build()
}
