package postgres

import (
	"context"
	"fmt"

	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &vshnPostgresBackupStorage{}

// Get returns a VSHNPostgresBackupStorage service based on StackGres/CNPG backup resource
func (v *vshnPostgresBackupStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnpostgresql.ListVSHNPostgreSQL(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances")
	}

	var vshnBackup *v1.VSHNPostgresBackup
	for _, value := range instances.Items {
		client, err := v.vshnpostgresql.GetDynKubeClient(ctx, &value)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig: %w", err)
		}

		targetSchema := DetermineTargetSchema(value.Spec.CompositionRef.Name)
		backupInfo, err := v.backups.GetBackup(ctx, name, value.Status.InstanceNamespace, targetSchema, client)
		if err != nil {
			resolvedErr := apiserver.ResolveError(targetSchema.GroupResource(), err)
			if apierrors.IsNotFound(resolvedErr) {
				continue
			}
			return nil, err
		}

		vshnBackup = v1.NewVSHNPostgresBackup(backupInfo, value.GetName(), namespace)
	}

	if vshnBackup == nil {
		return nil, apierrors.NewNotFound(v1.New().GetGroupVersionResource().GroupResource(), name)
	}

	return vshnBackup, nil
}

// Get schema.GroupVersionResource for either StackGres or CNPG backups according to the compositionReference.
// Defaults to StackGres.
func DetermineTargetSchema(compositionRefName string) schema.GroupVersionResource {
	targetSchema := SGbackupGroupVersionResource
	if compositionRefName == "vshnpostgrescnpg.vshn.appcat.vshn.io" {
		targetSchema = CNPGbackupGroupVersionResource
	}

	return targetSchema
}
