package webhooks

import (
	mysqlv1alpha1 "github.com/vshn/appcat/v4/apis/sql/mysql/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-mysql-sql-crossplane-io-v1alpha1-database,mutating=false,failurePolicy=fail,groups="mysql.sql.crossplane.io",resources=databases,versions=v1alpha1,name=databases.mysql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1
//+kubebuilder:webhook:verbs=delete,path=/validate-mysql-sql-crossplane-io-v1alpha1-grant,mutating=false,failurePolicy=fail,groups="mysql.sql.crossplane.io",resources=grants,versions=v1alpha1,name=grants.mysql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1
//+kubebuilder:webhook:verbs=delete,path=/validate-mysql-sql-crossplane-io-v1alpha1-user,mutating=false,failurePolicy=fail,groups="mysql.sql.crossplane.io",resources=users,versions=v1alpha1,name=users.mysql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbdatabaseinstances,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=syn.tools,resources=compositemariadbuserinstances,verbs=get;list;watch;patch;update

// SetupMysqlDatabaseDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupMysqlDatabaseDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&mysqlv1alpha1.Database{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("mysql_database"),
		}).
		Complete()
}

// SetupMysqlGrantDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupMysqlGrantDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&mysqlv1alpha1.Grant{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("mysql_grant"),
		}).
		Complete()
}

// SetupMysqlUserDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupMysqlUserDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&mysqlv1alpha1.User{}).
		WithValidator(&GenericDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("mysql_grant"),
		}).
		Complete()
}
