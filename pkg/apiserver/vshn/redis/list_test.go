package redis

import (
	"context"
	"testing"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/utils/ptr"
)

func Test_vshnRedisBackupStorage_List(t *testing.T) {
	tests := []struct {
		name      string
		instances *vshnv1.VSHNRedisList
		snapshots *k8upv1.SnapshotList
		wantNs    string
		want      runtime.Object
		wantDate  metav1.Time
		wantErr   bool
	}{
		{
			name: "GivenAvailableBackups_ThenExpectBackupList",
			instances: &vshnv1.VSHNRedisList{
				Items: []vshnv1.VSHNRedis{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "myinstance",
							Namespace: "ns1",
						},
						Status: vshnv1.VSHNRedisStatus{
							InstanceNamespace: "ins1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mysecondinstance",
							Namespace: "ns1",
						},
						Status: vshnv1.VSHNRedisStatus{
							InstanceNamespace: "ins2",
						},
					},
				},
			},
			snapshots: &k8upv1.SnapshotList{
				Items: []k8upv1.Snapshot{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "snap1",
							Namespace: "ins1",
						},
						Spec: k8upv1.SnapshotSpec{
							ID: ptr.To("snap1"),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "snap2",
							Namespace: "ins2",
						},
						Spec: k8upv1.SnapshotSpec{
							ID: ptr.To("snap2"),
						},
					},
				},
			},
			wantNs:   "ns1",
			wantDate: metav1.Date(2023, time.April, 3, 13, 37, 0, 0, time.UTC),
			want: &appcatv1.VSHNRedisBackupList{
				Items: []appcatv1.VSHNRedisBackup{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "snap1",
							Namespace: "ns1",
						},
						Status: appcatv1.VSHNRedisBackupStatus{
							ID:       "snap1",
							Instance: "myinstance",
							Date:     metav1.Date(2023, time.April, 3, 13, 37, 0, 0, time.UTC),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "snap2",
							Namespace: "ns1",
						},
						Status: appcatv1.VSHNRedisBackupStatus{
							ID:       "snap2",
							Instance: "mysecondinstance",
							Date:     metav1.Date(2023, time.April, 3, 13, 37, 0, 0, time.UTC),
						},
					},
				},
			},
		},
		{
			name: "GivenNobackups_ThenExpectEmptyList",
			instances: &vshnv1.VSHNRedisList{
				Items: []vshnv1.VSHNRedis{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "myinstance",
							Namespace: "ns1",
						},
						Status: vshnv1.VSHNRedisStatus{
							InstanceNamespace: "ins1",
						},
					},
				},
			},
			snapshots: &k8upv1.SnapshotList{
				Items: []k8upv1.Snapshot{},
			},
			wantNs: "ns1",
			want: &appcatv1.VSHNRedisBackupList{
				Items: []appcatv1.VSHNRedisBackup{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := request.WithNamespace(context.TODO(), tt.wantNs)

			for i := range tt.snapshots.Items {
				tt.snapshots.Items[i].Spec.Date = &tt.wantDate
			}

			v := &vshnRedisBackupStorage{
				snapshothandler: &mockhandler{
					snapshots: tt.snapshots,
				},
				vshnRedis: &mockprovider{
					instances: tt.instances,
				},
			}
			got, err := v.List(ctx, &internalversion.ListOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("vshnRedisBackupStorage.List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
