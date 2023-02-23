package postgres

import (
	"appcat-apiserver/apis/appcat/v1"
	mock_postgres "appcat-apiserver/apiserver/vshn/postgres/mock"
	"fmt"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestVSHNPostgresBackupStorage_List(t *testing.T) {
	tests := map[string]struct {
		postgresqls    *vshnv1.VSHNPostgreSQLList
		postgresqlsErr error

		backupInfoCalls func(mock_postgres.MocksgbackupProvider, error)
		backupInfosErr  error

		vshnBackups *v1.VSHNPostgresBackupList
		err         error
	}{
		"GivenPostgresDataAndListOfBackupInfos_ThenReturnVshnBackups": {
			postgresqls: vshnPostgreSQLInstances,
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error) {
				provider.EXPECT().
					ListSGBackup(gomock.Any(), "namespace-one", gomock.Any()).
					Return(&[]v1.SGBackupInfo{*backupInfoOne}, nil).
					Times(1)

				provider.EXPECT().
					ListSGBackup(gomock.Any(), "namespace-two", gomock.Any()).
					Return(&[]v1.SGBackupInfo{*backupInfoTwo}, err).
					Times(1)
			},
			vshnBackups: &v1.VSHNPostgresBackupList{
				Items: []v1.VSHNPostgresBackup{
					*vshnBackupOne,
					*vshnBackupTwo,
				},
			},
		},
		"GivenNoPostgresData_ThenReturnEmpty": {
			postgresqls:     &vshnv1.VSHNPostgreSQLList{},
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error) {},
			vshnBackups: &v1.VSHNPostgresBackupList{
				Items: []v1.VSHNPostgresBackup(nil),
			},
		},
		"GivenNoBackups_ThenReturnEmpty": {
			postgresqls: vshnPostgreSQLInstances,
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error) {
				provider.EXPECT().
					ListSGBackup(gomock.Any(), "namespace-one", gomock.Any()).
					Return(&[]v1.SGBackupInfo{}, nil).
					Times(1)

				provider.EXPECT().
					ListSGBackup(gomock.Any(), "namespace-two", gomock.Any()).
					Return(&[]v1.SGBackupInfo{}, err).
					Times(1)
			},
			vshnBackups: &v1.VSHNPostgresBackupList{
				Items: []v1.VSHNPostgresBackup(nil),
			},
		},
		"GivenBackupErrList_ThenReturnError": {
			postgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "postgres-two",
						},
						Status: vshnv1.VSHNPostgreSQLStatus{
							InstanceNamespace: "namespace-two",
						},
					},
				},
			},
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error) {
				provider.EXPECT().
					ListSGBackup(gomock.Any(), "namespace-two", gomock.Any()).
					Return(&[]v1.SGBackupInfo{}, err).
					Times(1)
			},
			backupInfosErr: fmt.Errorf("cannot get list"),
			err:            fmt.Errorf("cannot get list"),
		},
	}
	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			stor, backupsProvider, vshnPostgresProvider := newMockedVSHNPostgresBackupStorage(t, ctrl)

			vshnPostgresProvider.EXPECT().
				ListVSHNPostgreSQL(gomock.Any(), gomock.Any()).
				Return(tc.postgresqls, nil).
				Times(1)

			tc.backupInfoCalls(*backupsProvider, tc.backupInfosErr)

			actualList, err := stor.List(request.WithRequestInfo(
				request.WithNamespace(request.NewContext(), "namespace"),
				&request.RequestInfo{
					Verb:     "list",
					APIGroup: v1.GroupVersion.Group,
					Resource: v1.ResourceBackup,
				}), nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.vshnBackups, actualList)
		})
	}
}

type testWatcher struct {
	events     chan watch.Event
	countStops int
}

func (w *testWatcher) Stop() {
	w.countStops++
}

func (w *testWatcher) ResultChan() <-chan watch.Event {
	return w.events
}

func TestVSHNPostgresBackupStorage_Watch(t *testing.T) {
	tests := map[string]struct {
		postgresqls    *vshnv1.VSHNPostgreSQLList
		postgresqlsErr error

		unstructuredEvents []watch.Event
		unstructuredErr    error

		backupInfoCalls func(mock_postgres.MocksgbackupProvider, error, []watch.Interface)

		stopWatchCounterOne int
		stopWatchCounterTwo int

		vshnBackupEvents []watch.Event
		err              error
	}{
		"GivenSGBackupsUnstructured_ThenVSHNBackupsEvents": {
			postgresqls:         vshnPostgreSQLInstances,
			stopWatchCounterOne: 1,
			stopWatchCounterTwo: 1,
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, _ error, watches []watch.Interface) {
				provider.EXPECT().
					WatchSGBackup(gomock.Any(), "namespace-one", gomock.Any()).
					Return(watches[0], nil).
					AnyTimes()

				provider.EXPECT().
					WatchSGBackup(gomock.Any(), "namespace-two", gomock.Any()).
					Return(watches[1], nil).
					AnyTimes()
			},
			unstructuredEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: unstructuredBackupOne,
				},
				{
					Type:   watch.Modified,
					Object: unstructuredBackupTwo,
				},
			},
			vshnBackupEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: vshnBackupOne,
				},
				{
					Type:   watch.Modified,
					Object: vshnBackupTwo,
				},
			},
		},
		"GivenErrNotFound_ThenErrNotFound": {
			postgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "postgres-one",
						},
						Status: vshnv1.VSHNPostgreSQLStatus{
							InstanceNamespace: "namespace-one",
						},
					},
				},
			},
			unstructuredErr: apierrors.NewNotFound(schema.GroupResource{
				Resource: "sgbackups",
			}, "not-found"),
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error, _ []watch.Interface) {
				provider.EXPECT().
					WatchSGBackup(gomock.Any(), "namespace-one", gomock.Any()).
					Return(nil, err).
					AnyTimes()
			},
			err: apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.GroupVersion.Group,
				Resource: v1.ResourceBackup,
			}, "not-found"),
		},
		"GivenPostgresInstancesError_ThenError": {
			postgresqlsErr:  fmt.Errorf("cannot get postgres instances"),
			err:             fmt.Errorf("cannot list VSHNPostgreSQL instances"),
			backupInfoCalls: func(provider mock_postgres.MocksgbackupProvider, err error, _ []watch.Interface) {},
		},
	}

	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			stor, backupProvider, vshnPostgresProvider := newMockedVSHNPostgresBackupStorage(t, ctrl)

			backupWatcherOne := &testWatcher{
				events: make(chan watch.Event, 1),
			}
			backupWatcherTwo := &testWatcher{
				events: make(chan watch.Event, 1),
			}
			if tc.unstructuredEvents != nil {
				backupWatcherOne.events <- tc.unstructuredEvents[0]
				backupWatcherTwo.events <- tc.unstructuredEvents[1]

				close(backupWatcherOne.events)
				close(backupWatcherTwo.events)
			}

			vshnPostgresProvider.EXPECT().
				ListVSHNPostgreSQL(gomock.Any(), gomock.Any()).
				Return(tc.postgresqls, tc.postgresqlsErr).
				Times(1)

			tc.backupInfoCalls(*backupProvider, tc.unstructuredErr, []watch.Interface{backupWatcherOne, backupWatcherTwo})

			vshnBackupWatch, err := stor.Watch(request.WithRequestInfo(
				request.WithNamespace(request.NewContext(), "namespace"),
				&request.RequestInfo{
					Verb:     "watch",
					APIGroup: v1.GroupVersion.Group,
					Resource: v1.ResourceBackup,
				}), nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			e1 := <-vshnBackupWatch.ResultChan()
			e2 := <-vshnBackupWatch.ResultChan()
			vshnBackupWatch.Stop()
			vshnBackupEvents := []watch.Event{e1, e2}
			assert.Equal(t, tc.stopWatchCounterOne, backupWatcherOne.countStops)
			assert.Equal(t, tc.stopWatchCounterTwo, backupWatcherTwo.countStops)
			assert.ElementsMatch(t, tc.vshnBackupEvents, vshnBackupEvents)
		})
	}

}
