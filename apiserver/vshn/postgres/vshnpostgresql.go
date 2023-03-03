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
	ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error)
}

type kubeVSHNPostgresqlProvider struct {
	client.Client
}

// ListVSHNPostgreSQL fetches a list of VSHNPostgreSQL.
func (k *kubeVSHNPostgresqlProvider) ListVSHNPostgreSQL(ctx context.Context, namespace string) (*vshnv1.VSHNPostgreSQLList, error) {
	list := &vshnv1.VSHNPostgreSQLList{}
	err := k.Client.List(ctx, list, &client.ListOptions{
		Namespace: namespace,
	})
	return list, err
}
