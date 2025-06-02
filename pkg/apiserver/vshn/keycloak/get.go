package keycloak

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &vshnKeycloakBackupStorage{}

func (v *vshnKeycloakBackupStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from context")
	}

	instances, err := v.vshnKeycloak.ListVSHNkeycloak(ctx, namespace)
	if err != nil {
		return nil, err
	}

	keycloakBackup := &appcatv1.VSHNKeycloakBackup{}

	for _, instance := range instances.Items {
		pgNamespace, pgName := v.getPostgreSQLNamespaceAndName(ctx, &instance)
		dynClient, err := v.vshnKeycloak.GetDynKubeClient(ctx, &instance)
		if err != nil {
			return nil, err
		}
		sgBackups, err := v.sgBackup.ListSGBackup(ctx, pgNamespace, dynClient, &internalversion.ListOptions{})
		if err != nil {
			return nil, err
		}

		var sgBackup *appcatv1.SGBackupInfo
		for _, back := range *sgBackups {
			hashedName := hashString(back.GetName())
			if name == hashedName {
				sgBackup = &back
				break
			}
		}

		if sgBackup != nil {
			status := appcatv1.VSHNKeycloakBackupStatus{
				DatabaseBackupAvailable: true,
				DatabaseBackupStatus: appcatv1.VSHNPostgresBackupStatus{
					BackupInformation: &sgBackup.BackupInformation,
					Process:           &sgBackup.Process,
					DatabaseInstance:  pgName,
				},
			}

			backupMeta := sgBackup.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			keycloakBackup = &appcatv1.VSHNKeycloakBackup{
				ObjectMeta: backupMeta,
				Status:     status,
			}
		}
	}

	return keycloakBackup, nil
}
