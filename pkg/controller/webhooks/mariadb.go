package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnmariadb,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnmariadbs,versions=v1,name=vshnmariadb.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnmariadbs,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnmariadbs/status,verbs=get;list;watch;patch;update

var (
	mariadbGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNMariaDB"}
	mariadbGR = schema.GroupResource{Group: mariadbGK.Group, Resource: "vshnmariadb"}
)

var _ webhook.CustomValidator = &MariaDBWebhookHandler{}

type MariaDBWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupMariaDBWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupMariaDBWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNMariaDB{}).
		WithValidator(&MariaDBWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("mariadb"),
				withQuota,
				&vshnv1.VSHNMariaDB{},
				"mariadb",
				mariadbGK,
				mariadbGR,
			),
		}).
		Complete()
}
