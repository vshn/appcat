package postgres

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
	"github.com/vshn/appcat-apiserver/test/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_ListXVSHNPostgreSQL(t *testing.T) {
	tests := map[string]struct {
		namespace           string
		postgresqls         *vshnv1.XVSHNPostgreSQLList
		expectedPostgresqls *vshnv1.XVSHNPostgreSQLList
	}{
		"GivenAListOfPostgreSQLs_ThenFilter": {
			namespace: "namespace-prod",
			postgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{
					getInstance("prod", "namespace-prod"),
					getInstance("prod-2", "namespace-prod-2"),
					getInstanceWithoutLabels("prod-3"),
					getInstanceWithoutLabels("prod"),
					getInstanceWithoutClaimName("prod", "namespace-prod"),
					getInstanceWithoutClaimName("prod-3", "namespace-prod-2"),
					getInstanceWithoutClaimNamespace("prod"),
					getInstanceWithoutClaimNamespace("prod-3"),
					getInstance("test", "namespace-test-2"),
					getInstance("test", "namespace-prod"),
				},
			},
			expectedPostgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{
					getInstance("prod", "namespace-prod"),
					getInstance("test", "namespace-prod"),
				},
			},
		},
		"GivenAListOfPostgreSQLs_ThenFilter_2": {
			namespace: "namespace-not-match",
			postgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{
					getInstance("prod", "namespace-prod"),
					getInstance("prod-2", "namespace-prod-2"),
					getInstanceWithoutLabels("prod-3"),
					getInstanceWithoutLabels("prod"),
					getInstanceWithoutClaimName("prod", "namespace-prod"),
					getInstanceWithoutClaimName("prod-3", "namespace-prod-2"),
					getInstanceWithoutClaimNamespace("prod"),
					getInstanceWithoutClaimNamespace("prod-3"),
					getInstance("test", "namespace-test-2"),
					getInstance("test", "namespace-prod"),
				},
			},
			expectedPostgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{},
			},
		},
	}

	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {

			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := mocks.NewMockClient(ctrl)
			provider := kubeXVSHNPostgresqlProvider{
				client,
			}

			client.EXPECT().
				List(gomock.Any(), gomock.Any()).
				SetArg(1, *tc.postgresqls).
				Times(1)

			// WHEN
			instances, err := provider.ListXVSHNPostgreSQL(context.Background(), tc.namespace)

			// THEN
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPostgresqls, instances)
		})
	}
}

func getInstanceWithoutClaimName(name, namespace string) vshnv1.XVSHNPostgreSQL {
	return vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
			Labels: map[string]string{
				claimNamespaceLabel: namespace,
			},
		},
	}
}

func getInstanceWithoutClaimNamespace(name string) vshnv1.XVSHNPostgreSQL {
	return vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
			Labels: map[string]string{
				claimNameLabel: name,
			},
		},
	}
}

func getInstanceWithoutLabels(name string) vshnv1.XVSHNPostgreSQL {
	return vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
		},
	}
}

func getInstance(name, namespace string) vshnv1.XVSHNPostgreSQL {
	return vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
			Labels: map[string]string{
				claimNameLabel:      name,
				claimNamespaceLabel: namespace,
			},
		},
	}
}
