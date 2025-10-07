package postgres

import (
	"testing"

	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/test/mocks"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	client "k8s.io/client-go/dynamic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"k8s.io/apiserver/pkg/endpoints/request"
)

var (
	schemas = []schema.GroupVersionResource{
		SGbackupGroupVersionResource,
		CNPGbackupGroupVersionResource,
	}
)

func TestVSHNPostgresBackupStorage_Get(t *testing.T) {
	tests := map[string]struct {
		name               string
		postgresqls        *vshnv1.VSHNPostgreSQLList
		backupInfo         *v1.BackupInfo
		backupInfoCalls    func(mocks.MockbackupProvider, string)
		vshnPostgresBackup *v1.VSHNPostgresBackup
		err                error
		client             *client.DynamicClient
	}{
		"GivenAListOfPostgresAndBackups_ThenVSHNPostgresBackupStackGres": {
			name:        "one",
			postgresqls: vshnPostgreSQLInstances,
			backupInfo:  backupInfoOne,
			backupInfoCalls: func(provider mocks.MockbackupProvider, name string) {
				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-one", SGbackupGroupVersionResource, gomock.Nil()).
					Return(backupInfoOne, nil).
					Times(1)

				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-two", SGbackupGroupVersionResource, gomock.Nil()).
					Return(nil, apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), name)).
					Times(1)
			},
			vshnPostgresBackup: vshnBackupOne,
			err:                nil,
			client:             nil,
		},
		"GivenAListOfPostgresAndBackups_ThenVSHNPostgresBackupCNPG": {
			name:        "one",
			postgresqls: vshnPostgreSQLInstancesCnpg,
			backupInfo:  backupInfoOneCnpg,
			backupInfoCalls: func(provider mocks.MockbackupProvider, name string) {
				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-one", CNPGbackupGroupVersionResource, gomock.Nil()).
					Return(backupInfoOneCnpg, nil).
					Times(1)

				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-two", CNPGbackupGroupVersionResource, gomock.Nil()).
					Return(nil, apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), name)).
					Times(1)
			},
			vshnPostgresBackup: vshnBackupOneCnpg,
			err:                nil,
			client:             nil,
		},
		"GivenErrNotFound_ThenErrNotFoundStackGres": {
			name:        "one",
			postgresqls: vshnPostgreSQLInstances,
			backupInfo:  nil,
			backupInfoCalls: func(provider mocks.MockbackupProvider, name string) {
				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-one", SGbackupGroupVersionResource, gomock.Nil()).
					Return(nil, apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), name)).
					Times(1)

				provider.EXPECT().
					GetBackup(gomock.Any(), name, "namespace-two", SGbackupGroupVersionResource, gomock.Nil()).
					Return(nil, apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), name)).
					Times(1)
			},
			vshnPostgresBackup: nil,
			err:                apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), "one"),
			client:             nil,
		},
		"GivenNoPostgresInstances_ThenErrNotFound": {
			name:        "one",
			postgresqls: &vshnv1.VSHNPostgreSQLList{},
			backupInfo:  nil,
			backupInfoCalls: func(provider mocks.MockbackupProvider, name string) {
				for _, s := range schemas {
					provider.EXPECT().
						GetBackup(gomock.Any(), gomock.Any(), gomock.Any(), s, gomock.Nil()).
						Times(0)
				}
			},
			vshnPostgresBackup: nil,
			err:                apierrors.NewNotFound(v1.GetGroupResource(v1.ResourceBackup), "one"),
			client:             nil,
		},
	}

	for n, tc := range tests {

		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			s, backupProvider, postgresProvider := newMockedVSHNPostgresBackupStorage(t, ctrl)

			postgresProvider.EXPECT().
				ListVSHNPostgreSQL(gomock.Any(), gomock.Any()).
				Return(tc.postgresqls, nil).
				Times(1)

			postgresProvider.EXPECT().
				GetDynKubeClient(gomock.Any(), gomock.Any()).
				Return(tc.client, nil).
				Times(len(tc.postgresqls.Items))

			tc.backupInfoCalls(*backupProvider, tc.name)

			actual, err := s.Get(request.WithRequestInfo(
				request.WithNamespace(request.NewContext(), "namespace"),
				&request.RequestInfo{
					Verb:     "get",
					APIGroup: v1.GroupVersion.Group,
					Resource: v1.ResourceBackup,
					Name:     tc.name,
				}),
				tc.name, nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.vshnPostgresBackup, actual)
		})
	}
}
