package webhooks

import (
	"context"
	"encoding/json"
	"fmt"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// See https://book.kubebuilder.io/reference/markers/webhook for docs
//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-vshn-appcat-vshn-io-v1-vshnpostgresql,mutating=false,failurePolicy=fail,groups=vshn.appcat.vshn.io,resources=vshnpostgresqls,versions=v1,name=postgresql.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

//RBAC
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls/status,verbs=get;list;watch;patch;update

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;patch;update;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch;update;delete

var (
	pgGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNPostgreSQL"}
	pgGR = schema.GroupResource{Group: pgGK.Group, Resource: "vshnpostgresqls"}
)

var _ webhook.CustomValidator = &PostgreSQLWebhookHandler{}

var blocklist = map[string]string{
	"listen_addresses":      "",
	"port":                  "",
	"cluster_name":          "",
	"hot_standby":           "",
	"fsync":                 "",
	"full_page_writes":      "",
	"log_destination":       "",
	"logging_collector":     "",
	"max_replication_slots": "",
	"max_wal_senders":       "",
	"wal_keep_segments":     "",
	"wal_level":             "",
	"wal_log_hints":         "",
	"archive_mode":          "",
	"archive_command":       "",
}

// PostgreSQLWebhookHandler handles all quota webhooks concerning postgresql by vshn.
type PostgreSQLWebhookHandler struct {
	DefaultWebhookHandler
}

// SetupPostgreSQLWebhookHandlerWithManager registers the validation webhook with the manager.
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
			),
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

	err := validateVacuumRepack(pg.Spec.Parameters.Service.VacuumEnabled, pg.Spec.Parameters.Service.RepackEnabled)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field:  "spec.parameters.service",
			Detail: fmt.Sprintf("pg.Spec.Parameters.Service.VacuumEnabled and pg.Spec.Parameters.Service.RepackEnabled settings can't be both disabled: %s", err.Error()),
			Type:   field.ErrorTypeForbidden,
		})
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

	instancesError := p.checkGuaranteedAvailability(pg)

	allErrs = append(allErrs, instancesError...)

	// longest postfix is 26 chars for the sgbackup object (eg. "-952zx-2024-07-25-12-50-10"). Max SgBackup length is 56, therefore 30 characters is the maximum length
	err = p.validateResourceNameLength(pg.GetName(), 30)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten PostgreSQL name to 30 characters or less: %s",
				err.Error()),
			BadValue: pg.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	errList := validatePgConf(pg)
	if errList != nil {
		allErrs = append(allErrs, errList...)
	}

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

	if pg.DeletionTimestamp != nil {
		return nil, nil
	}

	err := validateVacuumRepack(pg.Spec.Parameters.Service.VacuumEnabled, pg.Spec.Parameters.Service.RepackEnabled)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field:  "spec.parameters.service",
			Detail: fmt.Sprintf("pg.Spec.Parameters.Service.VacuumEnabled and pg.Spec.Parameters.Service.RepackEnabled settings can't be both disabled: %s", err.Error()),
			Type:   field.ErrorTypeForbidden,
		})
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
	instancesError := p.checkGuaranteedAvailability(pg)

	allErrs = append(allErrs, instancesError...)

	// longest postfix is 26 chars for the sgbackup object (eg. "-952zx-2024-07-25-12-50-10"). Max SgBackup length is 56, therefore 30 characters is the maximum length
	err = p.validateResourceNameLength(pg.GetName(), 30)
	if err != nil {
		allErrs = append(allErrs, &field.Error{
			Field: ".metadata.name",
			Detail: fmt.Sprintf("Please shorten PostgreSQL name, currently it is: %s",
				err.Error()),
			BadValue: pg.GetName(),
			Type:     field.ErrorTypeTooLong,
		})
	}

	errList := validatePgConf(pg)
	if errList != nil {
		allErrs = append(allErrs, errList...)
	}

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
	s, err := utils.FetchSidecarsFromCluster(ctx, p.client, "vshnpostgresqlplans")
	if err != nil {
		return apierrors.NewInternalError(err), fieldErrs
	}

	resourcesSidecars, err := utils.GetAllSideCarsResources(s)
	if err != nil {
		return apierrors.NewInternalError(err), fieldErrs
	}

	p.addPathsToResources(&resources, false)

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

	resources.AddResources(resourcesSidecars)
	resources.MultiplyBy(instances)

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

func (p *PostgreSQLWebhookHandler) checkGuaranteedAvailability(pg *vshnv1.VSHNPostgreSQL) (fieldErrs field.ErrorList) {
	// service level and instances are verified in the CRD validation, therefore I skip checking them
	if pg.Spec.Parameters.Service.ServiceLevel == "guaranteed" && pg.Spec.Parameters.Instances < 2 {
		fieldErrs = append(fieldErrs, &field.Error{
			Field:    "spec.parameters.instances",
			Detail:   "PostgreSQL instances with service level Guaranteed Availability must have at least 2 replicas. Please set .spec.parameters.instances: [2,3]. Additional costs will apply, please refer to: https://products.vshn.ch/appcat/pricing.html",
			Type:     field.ErrorTypeInvalid,
			BadValue: pg.Spec.Parameters.Instances,
		})
	}
	return fieldErrs
}

// validate vacuum and repack settings
func validateVacuumRepack(vacuum, repack bool) error {
	if !vacuum && !repack {
		return fmt.Errorf("repack cannot be enabled without vacuum")
	}
	return nil
}

func validatePgConf(pg *vshnv1.VSHNPostgreSQL) (fErros field.ErrorList) {

	pgConfBytes := pg.Spec.Parameters.Service.PostgreSQLSettings

	pgConf := map[string]string{}
	if pgConfBytes.Raw != nil {
		err := json.Unmarshal(pgConfBytes.Raw, &pgConf)
		if err != nil {
			fErros = append(fErros, &field.Error{
				Field:    "spec.parameters.service.postgresqlSettings",
				Detail:   fmt.Sprintf("Error parsing pgConf: %s", err.Error()),
				Type:     field.ErrorTypeInvalid,
				BadValue: pgConfBytes,
			})
			return fErros
		}
	}

	for key := range pgConf {
		if _, ok := blocklist[key]; ok {
			fErros = append(fErros, &field.Error{
				Field:    fmt.Sprintf("spec.parameters.service.postgresqlSettings[%s]", key),
				Type:     field.ErrorTypeForbidden,
				BadValue: key,
				Detail:   "https://stackgres.io/doc/latest/api/responses/error/#postgres-blocklist",
			})
		}
	}
	return fErros
}
