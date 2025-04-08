package postgres

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnpostgresqls,verbs=get;list;watch

// vshnPostgresqlProvider is an abstraction to interact with the K8s API
type vshnPostgresqlProvider interface {
	ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error)
	apiserver.ClientConfigurator
}

type kubeVSHNPostgresqlProvider struct {
	apiserver.ClientConfigurator
}

// ListVSHNPostgreSQL fetches a list of XVSHNPostgreSQL.
func (k *kubeVSHNPostgresqlProvider) ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error) {
	list := &vshnv1.VSHNPostgreSQLList{}
	err := k.List(ctx, list, &client.ListOptions{Namespace: namespace})
	cleanedList := make([]vshnv1.VSHNPostgreSQL, 0)
	for _, p := range list.Items {
		// In some cases instance namespaces is missing and as a consequence all backups from the whole cluster
		// are being exposed creating a security issue - check APPCAT-563.
		if p.Status.InstanceNamespace != "" {
			cleanedList = append(cleanedList, p)
		}
	}
	list.Items = cleanedList
	return list, err
}
