package webhooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// This go generate and kube builder marker are only here for completeness sake.
// Unfortunately the webhook generator is not very configurable yet and some
// values cannot be changed. This will clash if/when more webhooks are generated.

// Generate the webhook manifest from the marker comment
// //go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen webhook paths=./... output:dir=../../../config/controller/pg-webhooks

// See https://book.kubebuilder.io/reference/markers/webhook for docs
//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnpostgresql,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=postgresqls,versions=v1,name=postgresql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

var (
	pgGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNPostgreSQL"}
	pgGR = schema.GroupResource{Group: pgGK.Group, Resource: "vshnpostgresqls"}
)

var _ webhook.CustomValidator = &PostgreSQLWebhookHandler{}

// PostgreSQLWebhookHandler handles all quota webhooks concerning postgresql by vshn.
type PostgreSQLWebhookHandler struct {
	client    client.Client
	log       logr.Logger
	withQuota bool
}

// SetupPostgreSQLWebhookHandlerWithManager registers the validation webhook with the manager.
func SetupPostgreSQLWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNPostgreSQL{}).
		WithValidator(&PostgreSQLWebhookHandler{
			client:    mgr.GetClient(),
			log:       mgr.GetLogger().WithName("webhook").WithName("postgresql"),
			withQuota: withQuota,
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PostgreSQLWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	pg, ok := obj.(*vshnv1.VSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	if p.withQuota {
		quotaErrs, fieldErrs := p.checkPostgreSQLQuotas(ctx, pg, true)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
		allErrs = append(allErrs, fieldErrs...)
	}

	instancesError := p.checkGuaranteedAvailability(ctx, pg)

	allErrs = append(allErrs, instancesError...)

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			pgGK,
			pg.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PostgreSQLWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {

	allErrs := field.ErrorList{}
	pg, ok := newObj.(*vshnv1.VSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	if p.withQuota {
		quotaErrs, fieldErrs := p.checkPostgreSQLQuotas(ctx, pg, false)
		if quotaErrs != nil {
			allErrs = append(allErrs, &field.Error{
				Field: "quota",
				Detail: fmt.Sprintf("quota check failed: %s",
					quotaErrs.Error()),
				BadValue: "*your namespace quota*",
				Type:     field.ErrorTypeForbidden,
			})
		}
		allErrs = append(allErrs, fieldErrs...)
	}
	instancesError := p.checkGuaranteedAvailability(ctx, pg)

	allErrs = append(allErrs, instancesError...)

	// We aggregate and return all errors at the same time.
	// So the user is aware of all broken parameters.
	// But at the same time, if any of these fail we cannot do proper quota checks anymore.
	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			pgGK,
			pg.GetName(),
			allErrs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *PostgreSQLWebhookHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// checkPostgreSQLQuotas will read the plan if it's set and then check if any other size parameters are overwriten
func (p *PostgreSQLWebhookHandler) checkPostgreSQLQuotas(ctx context.Context, pg *vshnv1.VSHNPostgreSQL, checkNamespaceQuota bool) (quotaErrs *apierrors.StatusError, fieldErrs field.ErrorList) {
	var fieldErr *field.Error
	instances := int64(pg.Spec.Parameters.Instances)
	resources := utils.Resources{}

	if pg.Spec.Parameters.Size.Plan != "" {
		var err error
		resources, err = utils.FetchPlansFromCluster(ctx, p.client, "vshnpostgresqlplans", pg.Spec.Parameters.Size.Plan)
		if err != nil {
			return apierrors.NewInternalError(err), fieldErrs
		}
	}

	resourcesSidecars, err := utils.GetAllSideCarsResources(ctx, p.client, "vshnpostgresqlplans")
	if err != nil {
		return apierrors.NewInternalError(err), fieldErrs
	}

	p.addPathsToResources(&resources)

	if pg.Spec.Parameters.Size.CPU != "" {
		resources.CPULimits, fieldErr = parseResource(resources.CPULimitsPath, pg.Spec.Parameters.Size.CPU, "not a valid cpu size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	if pg.Spec.Parameters.Size.Requests.CPU != "" {
		resources.CPURequests, fieldErr = parseResource(resources.CPURequestsPath, pg.Spec.Parameters.Size.Requests.CPU, "not a valid cpu size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	if pg.Spec.Parameters.Size.Memory != "" {
		resources.MemoryLimits, fieldErr = parseResource(resources.MemoryLimitsPath, pg.Spec.Parameters.Size.Memory, "not a valid memory size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	if pg.Spec.Parameters.Size.Requests.Memory != "" {
		resources.MemoryRequests, fieldErr = parseResource(resources.MemoryRequestsPath, pg.Spec.Parameters.Size.Requests.Memory, "not a valid memory size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	if pg.Spec.Parameters.Size.Disk != "" {
		resources.Disk, fieldErr = parseResource(resources.DiskPath, pg.Spec.Parameters.Size.Disk, "not a valid cpu size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	resources.MultiplyBy(instances)
	resources.AddByResource(resourcesSidecars)

	checker := quotas.NewQuotaChecker(
		p.client,
		pg.GetName(),
		pg.GetNamespace(),
		pg.Status.InstanceNamespace,
		resources,
		pgGR,
		pgGK,
		checkNamespaceQuota,
	)

	return checker.CheckQuotas(ctx), fieldErrs
}

func parseResource(childPath *field.Path, value, errMessage string) (resource.Quantity, *field.Error) {
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return quantity, field.Invalid(childPath, value, errMessage)
	}
	return quantity, nil
}

func (p *PostgreSQLWebhookHandler) addPathsToResources(r *utils.Resources) {
	basePath := field.NewPath("spec", "parameters", "size")

	r.CPULimitsPath = basePath.Child("cpu")
	r.CPURequestsPath = basePath.Child("requests", "cpu")
	r.MemoryLimitsPath = basePath.Child("memory")
	r.MemoryRequestsPath = basePath.Child("requests", "memory")
	r.DiskPath = basePath.Child("disk")
}

func (p *PostgreSQLWebhookHandler) checkGuaranteedAvailability(ctx context.Context, pg *vshnv1.VSHNPostgreSQL) (fieldErrs field.ErrorList) {
	// service level and instances are verified in the CRD validation, therefore I skip checking them
	if pg.Spec.Parameters.Service.ServiceLevel == "guaranteed" && pg.Spec.Parameters.Instances < 2 {
		fieldErrs = append(fieldErrs, &field.Error{
			Field:    "spec.parameters.instances",
			Detail:   "guaranteed PostgreSQL instances must be at least 2. Please set .spec.parameters.instances: [2,3]. Additional cost will apply, please refer to: https://products.vshn.ch/appcat/pricing.html",
			Type:     field.ErrorTypeInvalid,
			BadValue: pg.Spec.Parameters.Instances,
		})
	}
	return fieldErrs
}
