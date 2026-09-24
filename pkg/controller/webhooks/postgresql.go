package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// See https://book.kubebuilder.io/reference/markers/webhook for docs
//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnpostgresql,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnpostgresqls,versions=v1,name=postgresql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

// Protect the XVSHNPostgreSQL composite from having its compositionRef changed once set
// and block the provisioning of new StackGres instances.
//+kubebuilder:webhook:verbs=create;update,path=/validate-vshn-appcat-vshn-io-v1-xvshnpostgresql,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls,versions=v1,name=xvshnpostgresql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//RBAC
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls/status,verbs=get;list;watch;patch;update

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;patch;update;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch;update;delete

//+kubebuilder:rbac:groups=postgresql.sql.crossplane.io,resources=providerconfigs,verbs=get;list;watch;

const (
	maxPgResourceNameLength = 30

	cnpgCompositionRef           = "vshnpostgrescnpg.vshn.appcat.vshn.io"
	cnpgExtensionMinMajorVersion = "18"
)

var (
	pgGK  = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNPostgreSQL"}
	pgGR  = schema.GroupResource{Group: pgGK.Group, Resource: "vshnpostgresqls"}
	xpgGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "XVSHNPostgreSQL"}

	_ webhook.CustomValidator = &PostgreSQLWebhookHandler{}
	_ webhook.CustomValidator = &XVSHNPostgreSQLWebhookHandler{}
)

type pgValidator interface {
	// oldPg is nil on create
	validate(newPg, oldPg *vshnv1.VSHNPostgreSQL) field.ErrorList
}

type PostgreSQLWebhookHandler struct {
	DefaultWebhookHandler
}

// validatorFor picks the validator for the PostgreSQL implementation the instance runs on.
// An empty compositionRef means the default composition, which is CNPG.
func validatorFor(pg *vshnv1.VSHNPostgreSQL) pgValidator {
	if pg.Spec.CompositionRef.Name == "" || pg.Spec.CompositionRef.Name == cnpgCompositionRef {
		return cnpgValidator{}
	}
	return stackgresValidator{}
}

func SetupPostgreSQLWebhookHandlerWithManager(mgr ctrl.Manager, withQuota bool) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.VSHNPostgreSQL{}).
		WithValidator(&PostgreSQLWebhookHandler{
			DefaultWebhookHandler: *New(
				mgr.GetClient(),
				mgr.GetLogger().WithName("webhook").WithName("postgresql"),
				withQuota,
				&vshnv1.VSHNPostgreSQL{},
				"postgresql",
				pgGK,
				pgGR,
				maxPgResourceNameLength,
			),
		}).
		Complete()
}

// XVSHNPostgreSQLWebhookHandler validates the nested XVSHNPostgreSQL composite resource.
type XVSHNPostgreSQLWebhookHandler struct{}

// SetupXVSHNPostgreSQLWebhookHandlerWithManager registers the XVSHNPostgreSQL validation webhook.
func SetupXVSHNPostgreSQLWebhookHandlerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vshnv1.XVSHNPostgreSQL{}).
		WithValidator(&XVSHNPostgreSQLWebhookHandler{}).
		Complete()
}

func (x *XVSHNPostgreSQLWebhookHandler) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	newPg, ok := obj.(*vshnv1.XVSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid XVSHNPostgreSQL object")
	}

	allErrs := newFielErrors(newPg.Name, xpgGK)
	if err := validateNoNewStackGres(newPg.Spec.CompositionRef.Name); err != nil {
		allErrs.Add(err)
	}
	return nil, allErrs.Get()
}

func (x *XVSHNPostgreSQLWebhookHandler) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldPg, ok := oldObj.(*vshnv1.XVSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid XVSHNPostgreSQL object")
	}
	newPg, ok := newObj.(*vshnv1.XVSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid XVSHNPostgreSQL object")
	}

	if newPg.DeletionTimestamp != nil {
		return nil, nil
	}

	allErrs := newFielErrors(newPg.Name, xpgGK)
	if err := validateCompositionRefImmutability(oldPg.Spec.CompositionRef.Name, newPg.Spec.CompositionRef.Name); err != nil {
		allErrs.Add(err)
	}
	return nil, allErrs.Get()
}

func (x *XVSHNPostgreSQLWebhookHandler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (p *PostgreSQLWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return p.validatePostgreSQL(ctx, obj, nil, true)
}

func (p *PostgreSQLWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return p.validatePostgreSQL(ctx, newObj, oldObj, false)
}

func (p *PostgreSQLWebhookHandler) validatePostgreSQL(ctx context.Context, newObj, oldObj runtime.Object, isCreate bool) (admission.Warnings, error) {
	newPg, ok := newObj.(*vshnv1.VSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	allErrs := newFielErrors(newPg.Name, pgGK)

	// Validate provider config
	providerConfigErrs := p.ValidateProviderConfig(ctx, newPg)
	if len(providerConfigErrs) > 0 {
		allErrs.Add(providerConfigErrs...)
	}

	// Validate Vacuum and Repack settings
	if err := validateVacuumRepack(newPg.Spec.Parameters.Service.VacuumEnabled, newPg.Spec.Parameters.Service.RepackEnabled); err != nil {
		allErrs.Add(err)
	}

	// Validate quotas if enabled
	// we can't use the default validation here because pg has a
	// different API than the rest...
	if p.withQuota {
		quotaErrs, fieldErrs := p.checkPostgreSQLQuotas(ctx, newPg, isCreate)
		if quotaErrs != nil {
			allErrs.Add(field.Forbidden(field.NewPath("quota"), fmt.Sprintf("quota check failed: %s", quotaErrs.Error())))
		}
		allErrs.Add(fieldErrs...)
	}

	// Validate guaranteed availability
	allErrs.Add(p.DefaultWebhookHandler.checkGuaranteedAvailability(newPg)...)

	// Validate name length
	if err := p.validateResourceNameLength(newPg.GetName()); err != nil {
		allErrs.Add(err)
	}

	// Validate pinImageTag matches majorVersion
	if err := validatePinImageTag(
		newPg.Spec.Parameters.Maintenance.PinImageTag,
		newPg.Spec.Parameters.Service.MajorVersion,
	); err != nil {
		allErrs.Add(err)
	}

	// oldPg stays nil on create
	var oldPg *vshnv1.VSHNPostgreSQL

	if isCreate {
		// Block the provisioning of new StackGres instances
		if err := validateNoNewStackGres(newPg.Spec.CompositionRef.Name); err != nil {
			allErrs.Add(err)
		}
	} else {
		var ok bool
		oldPg, ok = oldObj.(*vshnv1.VSHNPostgreSQL)
		if !ok {
			return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
		}
		if newPg.DeletionTimestamp != nil {
			return nil, nil
		}

		// Do not allow changing compositionRef if it has been set previously.
		// When creating a new VSHNPostgresQL, crossplane will automatically set this field if unset.
		if err := validateCompositionRefImmutability(oldPg.Spec.CompositionRef.Name, newPg.Spec.CompositionRef.Name); err != nil {
			allErrs.Add(err)
		}

		// Check for disk downsizing
		if diskErr := p.DefaultWebhookHandler.ValidateDiskDownsizing(ctx, oldPg, newPg, p.gk.Kind); diskErr != nil {
			allErrs.Add(diskErr)
		}

		// Validate encryption changes
		newEncryption := &newPg.Spec.Parameters.Encryption
		oldEncryption := &oldPg.Spec.Parameters.Encryption
		fieldPath := "spec.parameters.encryption.enabled"
		if err := validatePostgreSQLEncryptionChanges(newEncryption, oldEncryption, fieldPath); err != nil {
			allErrs.Add(err)
		}
	}

	// Validate everything that depends on the PostgreSQL implementation in use
	allErrs.Add(validatorFor(newPg).validate(newPg, oldPg)...)

	return nil, allErrs.Get()
}

// checkPostgreSQLQuotas will read the plan if it's set and then check if any other size parameters are overwritten
func (p *PostgreSQLWebhookHandler) checkPostgreSQLQuotas(ctx context.Context, pg *vshnv1.VSHNPostgreSQL, checkNamespaceQuota bool) (quotaErrs *apierrors.StatusError, fieldErrs field.ErrorList) {
	var fieldErr *field.Error
	instances := int64(pg.Spec.Parameters.Instances)
	resources := utils.Resources{}

	// Fetch plans if specified
	if pg.Spec.Parameters.Size.Plan != "" {
		var err error
		resources, err = utils.FetchPlansFromCluster(ctx, p.client, "vshnpostgresqlplans", pg.Spec.Parameters.Size.Plan)
		if err != nil {
			return apierrors.NewInternalError(err), fieldErrs
		}
	}

	// Fetch sidecars from the cluster
	sidecars, err := utils.FetchSidecarsFromCluster(ctx, p.client, "vshnpostgresqlplans")
	if err != nil {
		return apierrors.NewInternalError(err), fieldErrs
	}

	// Aggregate resources from sidecars
	resourcesSidecars, err := utils.GetAllSideCarsResources(sidecars)
	if err != nil {
		return apierrors.NewInternalError(err), fieldErrs
	}

	p.addPathsToResources(&resources, false)

	// Parse and validate resource requests and limits
	if pg.Spec.Parameters.Size.CPU != "" {
		resources.CPULimits, fieldErr = parseResource(resources.CPULimitsPath, pg.Spec.Parameters.Size.CPU, "not a valid CPU size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	if pg.Spec.Parameters.Size.Requests.CPU != "" {
		resources.CPURequests, fieldErr = parseResource(resources.CPURequestsPath, pg.Spec.Parameters.Size.Requests.CPU, "not a valid CPU size")
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
		resources.Disk, fieldErr = parseResource(resources.DiskPath, pg.Spec.Parameters.Size.Disk, "not a valid disk size")
		if fieldErr != nil {
			fieldErrs = append(fieldErrs, fieldErr)
		}
	}

	// Add aggregated sidecar resources
	resources.AddResources(resourcesSidecars)
	resources.MultiplyBy(instances)

	// Perform quota checks
	checker := quotas.NewQuotaChecker(
		p.client,
		pg.GetName(),
		pg.GetNamespace(),
		pg.Status.InstanceNamespace,
		resources,
		pgGR,
		pgGK,
		checkNamespaceQuota,
		instances,
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

func validateVacuumRepack(vacuum, repack bool) *field.Error {
	if !vacuum && !repack {
		return field.Forbidden(
			field.NewPath("spec.parameters.service"),
			"pg.Spec.Parameters.Service.VacuumEnabled and pg.Spec.Parameters.Service.RepackEnabled settings can't be both disabled",
		)
	}
	return nil
}

// validatePgConf checks the PostgreSQL settings against the blocklist of the
// implementation in use. detail is the message reported for blocked settings.
func validatePgConf(pg *vshnv1.VSHNPostgreSQL, blocklist map[string]struct{}, detail string) field.ErrorList {
	allErrs := field.ErrorList{}
	pgConfBytes := pg.Spec.Parameters.Service.PostgreSQLSettings
	pgConf := map[string]string{}

	if pgConfBytes.Raw != nil {
		if err := json.Unmarshal(pgConfBytes.Raw, &pgConf); err != nil {
			return append(allErrs, field.Invalid(field.NewPath("spec.parameters.service.postgresqlSettings"), pgConfBytes, fmt.Sprintf("error parsing pgConf: %v", err)))
		}
	}

	for key := range pgConf {
		if _, blocked := blocklist[key]; blocked {
			allErrs = append(allErrs, field.Forbidden(field.NewPath(fmt.Sprintf("spec.parameters.service.postgresqlSettings[%s]", key)), detail))
		}
	}

	return allErrs
}

func validatePostgreSQLEncryptionChanges(newEncryption, oldEncryption *vshnv1.VSHNPostgreSQLEncryption, fieldPath string) *field.Error {
	// Check if encryption setting is being changed
	if newEncryption.Enabled != oldEncryption.Enabled {
		return field.Forbidden(
			field.NewPath(fieldPath),
			"encryption setting cannot be changed after instance creation. It can only be set during initial creation.",
		)
	}
	return nil
}

// validateNoNewStackGres blocks the provisioning of new StackGres instances.
// An empty compositionRef means the default composition, which is CNPG.
func validateNoNewStackGres(compositionRef string) *field.Error {
	if compositionRef != "" && compositionRef != cnpgCompositionRef {
		return field.Forbidden(
			field.NewPath("spec", "compositionRef"),
			"provisioning of new StackGres instances is not allowed",
		)
	}
	return nil
}

// validateCompositionRefImmutability returns a Forbidden error if the compositionRef name has changed after being set
func validateCompositionRefImmutability(oldRef, newRef string) *field.Error {
	if oldRef != "" && newRef != oldRef {
		return field.Forbidden(field.NewPath("spec", "compositionRef"), "compositionRef is immutable")
	}
	return nil
}

// validatePinImageTag validates that pinImageTag's major version matches the specified majorVersion
func validatePinImageTag(pinImageTag, majorVersion string) *field.Error {
	if pinImageTag == "" {
		return nil
	}

	// Extract major version from pinImageTag (e.g., "15.13" -> "15", "16.4" -> "16")
	pinMajor := pinImageTag
	if idx := strings.Index(pinImageTag, "."); idx > 0 {
		pinMajor = pinImageTag[:idx]
	}

	if pinMajor != majorVersion {
		return field.Invalid(
			field.NewPath("spec", "parameters", "maintenance", "pinImageTag"),
			pinImageTag,
			fmt.Sprintf("pinImageTag major version %q must match majorVersion %q", pinMajor, majorVersion),
		)
	}

	return nil
}
