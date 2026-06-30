package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshngarage,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshngarages,versions=v1,name=vshngarage.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshngarages,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshngarages/status,verbs=get;list;watch;patch;update

var (
	garageGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNGarage"}
	garageGR = schema.GroupResource{Group: garageGK.Group, Resource: "vshngarage"}
)

var _ webhook.CustomValidator = &GarageWebhookHandler{}

type GarageWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupGarageWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupGarageWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNGarage{}).
		WithValidator(&GarageWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("garage"),
				withQuota,
				&vshnv1.VSHNGarage{},
				"garage",
				garageGK,
				garageGR,
				maxResourceNameLength,
			),
		}).
		Complete()
}
