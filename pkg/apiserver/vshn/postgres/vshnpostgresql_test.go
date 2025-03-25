package postgres

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/test/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ListVSHNPostgreSQL(t *testing.T) {
	tests := map[string]struct {
		namespace           string
		postgresqls         *vshnv1.VSHNPostgreSQLList
		expectedPostgresqls *vshnv1.VSHNPostgreSQLList
	}{
		"GivenAListOfPostgreSQLs_ThenFilter": {
			namespace: "namespace-prod",
			postgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("test", "namespace-prod", "instance-namespace"),
				},
			},
			expectedPostgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("test", "namespace-prod", "instance-namespace"),
				},
			},
		},
		"GivenAListOfPostgreSQLs_ThenFilter_2": {
			namespace: "namespace-not-match",
			postgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{},
			},
			expectedPostgresqls: &vshnv1.VSHNPostgreSQLList{
				Items: []vshnv1.VSHNPostgreSQL{},
			},
		},
	}

	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := mocks.NewMockClient(ctrl)
			provider := kubeVSHNPostgresqlProvider{
				client,
				ClientConfigurator: &apiserver.KubeClient{},
			}

			client.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				SetArg(1, *tc.postgresqls).
				Times(1)

			// WHEN
			instances, err := provider.ListVSHNPostgreSQL(context.Background(), tc.namespace)

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPostgresqls, instances)
		})
	}
}

func getInstanceWithoutClaimName(name, namespace string) vshnv1.VSHNPostgreSQL {
	return vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
	}
}

func getInstanceWithoutClaimNamespace(name string) vshnv1.VSHNPostgreSQL {
	return vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
	}
}

func getInstanceWithoutLabels(name string) vshnv1.VSHNPostgreSQL {
	return vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
	}
}

func getInstanceWithoutInstanceNamespace(name, namespace string) vshnv1.VSHNPostgreSQL {
	return vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
	}
}

func getInstance(name, namespace, instanceNamespace string) vshnv1.VSHNPostgreSQL {
	return vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
		Status: vshnv1.VSHNPostgreSQLStatus{
			InstanceNamespace: instanceNamespace,
		},
	}
}
