package postgres

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/test/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("prod-2", "namespace-prod-2", "instance-namespace"),
					getInstanceWithoutLabels("prod-3"),
					getInstanceWithoutLabels("prod"),
					getInstanceWithoutClaimName("prod", "namespace-prod"),
					getInstanceWithoutClaimName("prod-3", "namespace-prod-2"),
					getInstanceWithoutClaimNamespace("prod"),
					getInstanceWithoutClaimNamespace("prod-3"),
					getInstance("test", "namespace-test-2", "instance-namespace"),
					getInstance("test", "namespace-prod", "instance-namespace"),
					getInstanceWithoutInstanceNamespace("test", "namespace-prod"),
				},
			},
			expectedPostgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("test", "namespace-prod", "instance-namespace"),
				},
			},
		},
		"GivenAListOfPostgreSQLs_ThenFilter_2": {
			namespace: "namespace-not-match",
			postgresqls: &vshnv1.XVSHNPostgreSQLList{
				Items: []vshnv1.XVSHNPostgreSQL{
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("prod", "namespace-prod", "instance-namespace"),
					getInstance("prod-2", "namespace-prod-2", "instance-namespace"),
					getInstanceWithoutLabels("prod-3"),
					getInstanceWithoutLabels("prod"),
					getInstanceWithoutClaimName("prod", "namespace-prod"),
					getInstanceWithoutClaimName("prod-3", "namespace-prod-2"),
					getInstanceWithoutClaimNamespace("prod"),
					getInstanceWithoutClaimNamespace("prod-3"),
					getInstance("test", "namespace-test-2", "instance-namespace"),
					getInstance("test", "namespace-prod", "instance-namespace"),
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

func getInstanceWithoutInstanceNamespace(name, namespace string) vshnv1.XVSHNPostgreSQL {
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

func getInstance(name, namespace, instanceNamespace string) vshnv1.XVSHNPostgreSQL {
	return vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-tty",
			Labels: map[string]string{
				claimNameLabel:      name,
				claimNamespaceLabel: namespace,
			},
		},
		Status: vshnv1.XVSHNPostgreSQLStatus{
			VSHNPostgreSQLStatus: vshnv1.VSHNPostgreSQLStatus{
				InstanceNamespace: instanceNamespace,
			},
		},
	}
}
