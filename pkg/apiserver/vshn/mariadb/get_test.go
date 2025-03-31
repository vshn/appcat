package mariadb

import (
	"context"
	"testing"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/utils/ptr"
)

func Test_vshnMariaDBBackupStorage_Get(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		instances    *vshnv1.VSHNMariaDBList
		snapshot     *k8upv1.Snapshot
		wantDate     metav1.Time
		wantNs       string
		want         runtime.Object
		wantErr      bool
	}{
		{
			name: "GivenExistingBackup_ThenExpectBackup",
			instances: &vshnv1.VSHNMariaDBList{
				Items: []vshnv1.VSHNMariaDB{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "myinstance",
							Namespace: "ns1",
						},
						Status: vshnv1.VSHNMariaDBStatus{
							InstanceNamespace: "ins1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "myinstance",
							Namespace: "ns2",
						},
						Status: vshnv1.VSHNMariaDBStatus{
							InstanceNamespace: "ins2",
						},
					},
				},
			},
			snapshot: &k8upv1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myid",
					Namespace: "ns1",
				},
				Spec: k8upv1.SnapshotSpec{
					ID: ptr.To("myid"),
				},
			},
			wantDate: metav1.Date(2023, time.April, 3, 13, 37, 0, 0, time.UTC),
			wantNs:   "ns1",
			want: &appcatv1.VSHNMariaDBBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myid",
					Namespace: "ns1",
				},
				Status: appcatv1.VSHNMariaDBBackupStatus{
					ID:       "myid",
					Instance: "myinstance",
					Date:     metav1.Date(2023, time.April, 3, 13, 37, 0, 0, time.UTC),
				},
			},
		},
		{
			name:      "GivenNoBackup_ThenExpectEmptyObjects",
			instances: &vshnv1.VSHNMariaDBList{},
			snapshot:  nil,
			want:      &appcatv1.VSHNMariaDBBackup{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := request.WithNamespace(context.TODO(), tt.wantNs)

			if tt.snapshot != nil {
				tt.snapshot.Spec.Date = &tt.wantDate
			}

			v := &vshnMariaDBBackupStorage{
				vshnMariaDB: &mockprovider{
					instances: tt.instances,
				},
				snapshothandler: &mockhandler{
					snapshot: tt.snapshot,
				},
			}
			got, err := v.Get(ctx, tt.instanceName, &metav1.GetOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("vshnMariaDBBackupStorage.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, got)

		})
	}
}
