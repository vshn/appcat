package vshnmariadb

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	my1alpha1 "github.com/vshn/appcat/v4/apis/sql/mysql/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserManagement(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetDesiredComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	// Nothing defined, let's return early
	if len(comp.Spec.Parameters.Service.Access) == 0 {
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

func addUser(comp common.Composite, svc *runtime.ServiceRuntime, username string) string {
	secretName, err := common.AddGenericSecret(comp, svc, "userpass-"+username, []string{"userpass"}, common.AllowDeletion)
	if err != nil {
		svc.Log.Error(err, "cannot deploy user password secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot deploy user password secret: %s", err)))
	}

	role := &my1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-role", comp.GetName(), username),
			Annotations: map[string]string{
				"crossplane.io/external-name": username,
			},
			Labels: map[string]string{
				runtime.ProviderConfigIgnoreLabel: "true",
				runtime.WebhookAllowDeletionLabel: "true",
			},
		},
		Spec: my1alpha1.UserSpec{
			ForProvider: my1alpha1.UserParameters{},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: comp.GetName(),
				},
			},
		},
	}

	// we need to get the secret object to mark it as deletable
	obj := &xkube.Object{}
	err = svc.GetDesiredComposedResourceByName(obj, secretName)
	if err != nil {
		svc.Log.Error(err, "cannot get user password secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get user password secret: %s", err)))
	}

	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}

	obj.Labels[runtime.WebhookAllowDeletionLabel] = "true"

	err = svc.SetDesiredComposedResource(obj)
	if err != nil {
		svc.Log.Error(err, "cannot set allow deletion label on user password secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set allow deletion label on user password secret: %s", err)))
	}

	err = svc.SetDesiredComposedResource(role,
		runtime.ComposedOptionProtects(comp.GetName()+"-provider-conf-credentials"),
		runtime.ComposedOptionProtects(secretName))
	if err != nil {
		svc.Log.Error(err, "cannot apply user")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply user: %s", err)))
	}

	return secretName
}

func addConnectionDetail(comp common.Composite, svc *runtime.ServiceRuntime, secretName, username, dbname string, connectionDetailRef *xpv1.SecretReference) {
	userpassCD, err := svc.GetObservedComposedResourceConnectionDetails(secretName)
	if err != nil {
		svc.Log.Error(err, "cannot get userpassword from secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get userpassword from secret: %s", err)))
	}

	compositeCD := svc.GetConnectionDetails()

	url := fmt.Sprintf("mysql://%s:%s@%s:%s", username, userpassCD["userpass"], compositeCD["MARIADB_HOST"], compositeCD["MARIADB_PORT"])

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
			"MARIADB_USERNAME": []byte(username),
			"MARIADB_PASSWORD": userpassCD["userpass"],
			"MARIADB_DB":       []byte(dbname),
			"MARIADB_HOST":     compositeCD["MARIADB_HOST"],
			"MARIADB_PORT":     compositeCD["MARIADB_PORT"],
			"MARIADB_URL":      []byte(url),
			"ca.crt":           compositeCD["ca.crt"],
		},
	}

	err = svc.SetDesiredKubeObject(userpassSecret, fmt.Sprintf("%s-user-%s", comp.GetName(), username),
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionDeployOnControlPlane)
	if err != nil {
		svc.Log.Error(err, "cannot get userpassword from secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot get userpassword from secret: %s", err)))
	}
}

func addProviderConfig(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) {
	cd := svc.GetConnectionDetails()

	endpoint := cd["MARIADB_HOST"]
	if _, exists := cd["LOADBALANCER_IP"]; exists {
		endpoint = cd["LOADBALANCER_IP"]
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-provider-conf-credentials",
			Namespace: svc.GetCrossplaneNamespace(),
		},
		Data: map[string][]byte{
			"username": cd["MARIADB_USERNAME"],
			"password": cd["MARIADB_PASSWORD"],
			"endpoint": endpoint,
			"port":     cd["MARIADB_PORT"],
		},
	}

	opts := []runtime.KubeObjectOption{
		runtime.KubeOptionProtects(comp.GetName() + "-instancens"),
		runtime.KubeOptionProtects(comp.GetName() + "-release"),
		runtime.KubeOptionProtects(comp.GetName() + "-netpol"),
		runtime.KubeOptionProtects(comp.GetName() + "-main-service"),
		runtime.KubeOptionDeployOnControlPlane,
		runtime.KubeOptionAllowDeletion,
	}

	if comp.GetInstances() != 1 {
		opts = append(opts, runtime.KubeOptionProtects(comp.GetName()+"-proxysql-sts"))
	}

	err := svc.SetDesiredKubeObject(secret, comp.GetName()+"-provider-conf-credentials", opts...)
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set credential secret for provider-sql: %s", err)))
		svc.Log.Error(err, "cannot set credential secret for provider-sql")
	}

	tls := "preferred"
	if comp.Spec.Parameters.TLS.TLSEnabled {
		// We need to skip-verify here as with the postgresql.
		// provider-sql does not support custom CA certs.
		tls = "skip-verify"
	}

	config := &my1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: my1alpha1.ProviderConfigSpec{
			TLS: &tls,
			Credentials: my1alpha1.ProviderCredentials{
				Source: "MySQLConnectionSecret",
				ConnectionSecretRef: &xpv1.SecretReference{
					Name:      comp.GetName() + "-provider-conf-credentials",
					Namespace: svc.GetCrossplaneNamespace(),
				},
			},
		},
	}

	err = svc.SetDesiredKubeObject(config, comp.GetName()+"-providerconfig",
		runtime.KubeOptionDeployOnControlPlane,
		runtime.KubeOptionAllowDeletion)
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot apply the provider config for provider sql: %s", err)))
		svc.Log.Error(err, "cannot apply the provider config for provider sql")
	}
}

// We check if the database is already specified.
// If not it will be added.
// This should handle cases where there are mutliple users pointing to the same
// database, and one is deleted, that the database is not dropped.
func addDatabase(comp common.Composite, svc *runtime.ServiceRuntime, name string) {
	resname := fmt.Sprintf("%s-%s-database", comp.GetName(), name)

	xdb := &my1alpha1.Database{}

	// If there's a database with the same name we will just return
	err := svc.GetDesiredComposedResourceByName(xdb, resname)
	if err == nil {
		return
	}
	if err != runtime.ErrNotFound {
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot check if database exists: %s", err)))
		svc.Log.Error(err, "cannot check if database exists")
	}

	xdb = &my1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: resname,
			Annotations: map[string]string{
				"crossplane.io/external-name": name,
			},
			Labels: map[string]string{
				runtime.ProviderConfigIgnoreLabel: "true",
				runtime.WebhookAllowDeletionLabel: "true",
			},
		},
		Spec: my1alpha1.DatabaseSpec{
			ForProvider: my1alpha1.DatabaseParameters{},
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

func addGrants(comp common.Composite, svc *runtime.ServiceRuntime, username, dbname string, privileges []string) {
	privs := []my1alpha1.GrantPrivilege{}

	if len(privileges) == 0 {
		privs = append(privs, "ALL")
	}

	for _, priv := range privileges {
		privs = append(privs, my1alpha1.GrantPrivilege(priv))
	}

	grant := &my1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-%s-grants", comp.GetName(), username, dbname),
			Labels: map[string]string{
				runtime.ProviderConfigIgnoreLabel: "true",
				runtime.WebhookAllowDeletionLabel: "true",
			},
		},
		Spec: my1alpha1.GrantSpec{
			ForProvider: my1alpha1.GrantParameters{
				Privileges: privs,
				User:       &username,
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
