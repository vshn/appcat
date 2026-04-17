package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnopenbao,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnopenbaos,versions=v1,name=vshnopenbao.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnopenbaos,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnopenbaos/status,verbs=get;list;watch;patch;update

var (
	openBaoGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNOpenBao"}
	openBaoGR = schema.GroupResource{Group: openBaoGK.Group, Resource: "vshnopenbaos"}
)

var _ webhook.CustomValidator = &OpenBaoWebhookHandler{}

type OpenBaoWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupOpenBaoWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupOpenBaoWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNOpenBao{}).
		WithValidator(&OpenBaoWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("openbao"),
				withQuota,
				&vshnv1.VSHNOpenBao{},
				"openbao",
				openBaoGK,
				openBaoGR,
				maxResourceNameLength,
			),
		}).
		Complete()
}
