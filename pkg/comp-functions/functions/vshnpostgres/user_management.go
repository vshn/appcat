package vshnpostgres

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	pgv1alpha1 "github.com/vshn/appcat/v4/apis/sql/postgresql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func UserManagement(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	permCount := 0
	permCount = permCount + len(comp.Spec.Parameters.Service.Users.Users)
	permCount = permCount + len(comp.Spec.Parameters.Service.Databases.Databases)
	permCount = permCount + len(comp.Spec.Parameters.Service.Grants)

	// Nothing defined, let's return early
	if permCount == 0 {
		return nil
	}

	cd := svc.GetConnectionDetails()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider-conf-credentials",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"username": cd["POSTGRESQL_USER"],
			"password": cd["POSTGRESQL_PASSWORD"],
			"endpoint": cd["POSTGRESQL_HOST"],
			"port":     cd["POSTGRESQL_PORT"],
			// TODO: we should probably generate something here...
			"userpass": []byte("test"),
		},
	}

	// TODO: we need to make sure that this doesn't get deleted before the actual roles/database/grants
	// when the user deletes the claim or they will get stuck with errors.
	err = svc.SetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("Cannot set credential secret for provider-sql: %s", err))
	}

	config := &pgv1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: pgv1alpha1.ProviderConfigSpec{
			// TODO: for a proper implementation this should be verify-full
			SSLMode: ptr.To("require"),
			Credentials: pgv1alpha1.ProviderCredentials{
				Source: "PostgreSQLConnectionSecret",
				ConnectionSecretRef: &xpv1.SecretReference{
					Name:      "provider-conf-credentials",
					Namespace: comp.GetInstanceNamespace(),
				},
			},
		},
	}

	err = svc.SetDesiredKubeObject(config, comp.GetName()+"-providerconfig")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("Cannot apply the provider config for provider sql: %s", err))
	}

	for _, user := range comp.Spec.Parameters.Service.Users.Users {
		role := &pgv1alpha1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", comp.GetName(), user),
				Annotations: map[string]string{
					"crossplane.io/external-name": user,
				},
			},
			Spec: pgv1alpha1.RoleSpec{
				ForProvider: pgv1alpha1.RoleParameters{
					Privileges: pgv1alpha1.RolePrivilege{
						Login: ptr.To(true),
					},
					PasswordSecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "provider-conf-credentials",
							Namespace: comp.Status.InstanceNamespace,
						},
						Key: "userpass",
					},
				},
				ResourceSpec: xpv1.ResourceSpec{
					ProviderConfigReference: &xpv1.Reference{
						Name: comp.GetName(),
					},
				},
			},
		}
		err := svc.SetDesiredComposedResource(role)
		if err != nil {
			svc.Log.Error(err, "cannot apply user")
		}
	}

	return nil
}
