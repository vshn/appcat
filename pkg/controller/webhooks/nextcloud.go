package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnnextcloud,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnnextclouds,versions=v1,name=vshnnextcloud.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnnextclouds,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnnextclouds/status,verbs=get;list;watch;patch;update

var (
	nextcloudGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNNextcloud"}
	nextcloudGR = schema.GroupResource{Group: nextcloudGK.Group, Resource: "vshnnextcloud"}
)

var _ webhook.CustomValidator = &TestWebhookHandler{}

type TestWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupNextcloudWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupNextcloudWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNNextcloud{}).
		WithValidator(&TestWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("nextcloud"),
				withQuota,
				&vshnv1.VSHNNextcloud{},
				"nextcloud",
				nextcloudGK,
				nextcloudGR,
			),
		}).
		Complete()
}
