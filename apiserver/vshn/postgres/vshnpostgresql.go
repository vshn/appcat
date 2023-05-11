package postgres

import (
	"context"
	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	claimNamespaceLabel = "crossplane.io/claim-namespace"
	claimNameLabel      = "crossplane.io/claim"
)

// vshnPostgresqlProvider is an abstraction to interact with the K8s API
type vshnPostgresqlProvider interface {
	ListXVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.XVSHNPostgreSQLList, error)
}

type kubeXVSHNPostgresqlProvider struct {
	client.Client
}

// ListXVSHNPostgreSQL fetches a list of XVSHNPostgreSQL.
func (k *kubeXVSHNPostgresqlProvider) ListXVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.XVSHNPostgreSQLList, error) {
	list := &vshnv1.XVSHNPostgreSQLList{}
	err := k.Client.List(ctx, list)
	cleanedList := make([]vshnv1.XVSHNPostgreSQL, 0)
	for _, p := range list.Items {
		if p.Labels[claimNamespaceLabel] == "" || p.Labels[claimNameLabel] == "" {
			continue
		}
		if p.Labels[claimNamespaceLabel] == namespace {
			cleanedList = append(cleanedList, p)
		}
	}
	list.Items = cleanedList
	return list, err
}
