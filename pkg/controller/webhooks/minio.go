package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnminio,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnminios,versions=v1,name=vshnminio.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnminios/status,verbs=get;list;watch;patch;update

var (
	minioGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNMinio"}
	minioGR = schema.GroupResource{Group: minioGK.Group, Resource: "vshnminio"}
)

var _ webhook.CustomValidator = &MinioWebhookHandler{}

type MinioWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupMinioWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupMinioWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNMinio{}).
		WithValidator(&MinioWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("minio"),
				withQuota,
				&vshnv1.VSHNMinio{},
				"minio",
				minioGK,
				minioGR,
			),
		}).
		Complete()
}
