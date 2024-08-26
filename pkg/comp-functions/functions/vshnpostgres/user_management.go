package vshnpostgres

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	pgv1alpha1 "github.com/vshn/appcat/v4/apis/sql/postgresql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func UserManagement(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetDesiredComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	// Nothing defined, let's return early
	if comp.Spec.Parameters.Service.Access == nil || len(comp.Spec.Parameters.Service.Access) == 0 {
		return nil
	}

	addProviderConfig(comp, svc)

	for _, access := range comp.Spec.Parameters.Service.Access {

		userPasswordRef := addUser(comp, svc, *access.User)

		dbname := *access.User
		if access.Database != nil {
			dbname = *access.Database
		}

		addDatabase(comp, svc, dbname)

		addGrants(comp, svc, *access.User, dbname, access.Privileges)

		addConnectionDetail(comp, svc, userPasswordRef, *access.User, dbname, access.WriteConnectionSecretToReference)
	}

	return nil
}

func addUser(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, username string) string {
	secretName, err := common.AddGenericSecret(comp, svc, "userpass-"+username, []string{"userpass"})
	if err != nil {
		svc.Log.Error(err, "cannot deploy user password secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot deploy user password secret: %s", err)))
	}

	role := &pgv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-role", comp.GetName(), username),
			Annotations: map[string]string{
				"crossplane.io/external-name": username,
			},
		},
		Spec: pgv1alpha1.RoleSpec{
			ForProvider: pgv1alpha1.RoleParameters{
				Privileges: pgv1alpha1.RolePrivilege{
					Login: ptr.To(true),
				},
				PasswordSecretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      secretName,
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

	err = svc.SetDesiredComposedResource(role, runtime.ComposedOptionProtects(comp.GetName()+"-provider-conf-credentials"), runtime.ComposedOptionProtects(secretName))
	if err != nil {
		svc.Log.Error(err, "cannot apply user")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply user: %s", err)))
	}

	return secretName
}

func addConnectionDetail(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, secretName, username, dbname string, connectionDetailRef *xpv1.SecretReference) {
	userpassCD, err := svc.GetObservedComposedResourceConnectionDetails(secretName)
	if err != nil {
		svc.Log.Error(err, "cannot get userpassword from secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get userpassword from secret: %s", err)))
	}

	compositeCD := svc.GetConnectionDetails()

	url := getPostgresURLCustomUser(compositeCD, string(userpassCD["userpass"]), username)

	om := metav1.ObjectMeta{
		Name:      comp.GetLabels()["crossplane.io/claim-name"] + "-" + username,
		Namespace: comp.GetClaimNamespace(),
	}
	if connectionDetailRef != nil {
		om.Name = connectionDetailRef.Name
		om.Namespace = connectionDetailRef.Namespace
	}

	userpassSecret := &corev1.Secret{
		ObjectMeta: om,
		Type:       corev1.SecretType("connection.crossplane.io/v1alpha1"),
		Data: map[string][]byte{
			"POSTGRESQL_USER":     []byte(username),
			"POSTGRESQL_PASSWORD": userpassCD["userpass"],
			"POSTGRESQL_DB":       []byte(dbname),
			"POSTGRESQL_HOST":     compositeCD["POSTGRESQL_HOST"],
			"POSTGRESQL_PORT":     compositeCD["POSTGRESQL_PORT"],
			"POSTGRESQL_URL":      []byte(url),
			"ca.crt":              compositeCD["ca.crt"],
			"tls.crt":             compositeCD["tls.crt"],
			"tls.key":             compositeCD["tls.key"],
		},
	}

	err = svc.SetDesiredKubeObject(userpassSecret, fmt.Sprintf("%s-user-%s", comp.GetName(), username))
	if err != nil {
		svc.Log.Error(err, "cannot get userpassword from secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get userpassword from secret: %s", err)))
	}
}

func addProviderConfig(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) {
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
		},
	}

	err := svc.SetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials",
		runtime.KubeOptionProtects("namespace-conditions"),
		runtime.KubeOptionProtects("cluster"),
		runtime.KubeOptionProtects(comp.GetName()+"-netpol"))
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set credential secret for provider-sql: %s", err)))
		svc.Log.Error(err, "cannot set credential secret for provider-sql")
	}

	config := &pgv1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: pgv1alpha1.ProviderConfigSpec{
			// Porvider-SQL doesn't support passing certificates to the config
			// se we're stuck with require, which doesn't actually verify the certs.
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
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply the provider config for provider sql: %s", err)))
		svc.Log.Error(err, "cannot apply the provider config for provider sql")
	}
}

// We check if the database is already specified.
// If not it will be added.
// This should handle cases where there are mutliple users pointing to the same
// database, and one is deleted, that the database is not dropped.
func addDatabase(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, name string) {
	resname := fmt.Sprintf("%s-%s-database", comp.GetName(), name)

	xdb := &pgv1alpha1.Database{}

	// If there's a database with the same name we will just return
	err := svc.GetDesiredComposedResourceByName(xdb, resname)
	if err == nil {
		return
	}
	if err != runtime.ErrNotFound {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot check if database exists: %s", err)))
		svc.Log.Error(err, "cannot check if database exists")
	}

	xdb = &pgv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: resname,
			Annotations: map[string]string{
				"crossplane.io/external-name": name,
			},
		},
		Spec: pgv1alpha1.DatabaseSpec{
			ForProvider: pgv1alpha1.DatabaseParameters{},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: comp.GetName(),
				},
			},
		},
	}

	err = svc.SetDesiredComposedResource(xdb, runtime.ComposedOptionProtects(comp.GetName()+"-provider-conf-credentials"))
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply database: %s", err)))
		svc.Log.Error(err, "cannot apply database")
	}
}

func addGrants(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, username, dbname string, privileges []string) {
	privs := []pgv1alpha1.GrantPrivilege{}

	if len(privileges) == 0 {
		privs = append(privs, "ALL")
	}

	for _, priv := range privileges {
		privs = append(privs, pgv1alpha1.GrantPrivilege(priv))
	}

	grant := &pgv1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), username, dbname),
		},
		Spec: pgv1alpha1.GrantSpec{
			ForProvider: pgv1alpha1.GrantParameters{
				Privileges: privs,
				Role:       &username,
				Database:   &dbname,
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: comp.GetName(),
				},
			},
		},
	}

	err := svc.SetDesiredComposedResource(grant, runtime.ComposedOptionProtects(comp.GetName()+"-provider-conf-credentials"))
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply database: %s", err)))
		svc.Log.Error(err, "cannot apply database")
	}
}
