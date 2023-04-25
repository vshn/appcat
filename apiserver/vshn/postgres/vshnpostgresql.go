package postgres

import (
	"context"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// vshnPostgresqlProvider is an abstraction to interact with the K8s API
//
//go:generate go run github.com/golang/mock/mockgen -source=$GOFILE -destination=./mock/$GOFILE
type vshnPostgresqlProvider interface {
	ListXVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.XVSHNPostgreSQLList, error)
}

type kubeXVSHNPostgresqlProvider struct {
	client.Client
}

// ListXVSHNPostgreSQL fetches a list of XVSHNPostgreSQL.
func (k *kubeXVSHNPostgresqlProvider) ListXVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.XVSHNPostgreSQLList, error) {
	list := &vshnv1.XVSHNPostgreSQLList{}
	err := k.Client.List(ctx, list, &client.ListOptions{
		Namespace: namespace,
	})
	return list, err
}
