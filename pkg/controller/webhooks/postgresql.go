package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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

//RBAC
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshnpostgresqls/status,verbs=get;list;watch;patch;update

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;patch;update;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch;update;delete

const (
	maxResourceNameLength = 30
)

var (
	pgGK = schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNPostgreSQL"}
	pgGR = schema.GroupResource{Group: pgGK.Group, Resource: "vshnpostgresqls"}

	_ webhook.CustomValidator = &PostgreSQLWebhookHandler{}

	blocklist = map[string]string{
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
)

type PostgreSQLWebhookHandler struct {
	DefaultWebhookHandler
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
			),
		}).
		Complete()
}

func (p *PostgreSQLWebhookHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return p.validatePostgreSQL(ctx, obj, nil, true)
}

func (p *PostgreSQLWebhookHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return p.validatePostgreSQL(ctx, newObj, oldObj, false)
}

func (p *PostgreSQLWebhookHandler) validatePostgreSQL(ctx context.Context, newObj, oldObj runtime.Object, isCreate bool) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	newPg, ok := newObj.(*vshnv1.VSHNPostgreSQL)
	if !ok {
		return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
	}

	// Validate provider config
	providerConfigErrs := p.DefaultWebhookHandler.ValidateProviderConfig(ctx, newPg)
	if len(providerConfigErrs) > 0 {
		allErrs = append(allErrs, providerConfigErrs...)
	}

	// Validate Vacuum and Repack settings
	if err := validateVacuumRepack(newPg.Spec.Parameters.Service.VacuumEnabled, newPg.Spec.Parameters.Service.RepackEnabled); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate quotas if enabled
	if p.withQuota {
		quotaErrs, fieldErrs := p.checkPostgreSQLQuotas(ctx, newPg, isCreate)
		if quotaErrs != nil {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("quota"), fmt.Sprintf("quota check failed: %s", quotaErrs.Error())))
		}
		allErrs = append(allErrs, fieldErrs...)
	}

	// Validate guaranteed availability
	allErrs = append(allErrs, p.checkGuaranteedAvailability(newPg)...)

	// Validate name length
	if err := p.validateResourceNameLength(newPg.GetName(), maxResourceNameLength); err != nil {
		allErrs = append(allErrs, field.TooLong(field.NewPath(".metadata.name"), newPg.GetName(), maxResourceNameLength))
	}

	// Validate PostgreSQL configuration
	allErrs = append(allErrs, validatePgConf(newPg)...)

	if !isCreate {
		oldPg, ok := oldObj.(*vshnv1.VSHNPostgreSQL)
		if !ok {
			return nil, fmt.Errorf("provided manifest is not a valid VSHNPostgreSQL object")
		}
		if newPg.DeletionTimestamp != nil {
			return nil, nil
		}

		// Enforce UseCNPG immutability
		if newPg.Spec.Parameters.UseCNPG != oldObj.(*vshnv1.VSHNPostgreSQL).Spec.Parameters.UseCNPG {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec.parameters.useCnpg"),
				"the method of deployment can only be set on creation",
			))
		}

		// Validate major upgrades
		if errList := validateMajorVersionUpgrade(newPg, oldPg); errList != nil {
			allErrs = append(allErrs, errList...)
		}

		// Validate encryption changes
		newEncryption := &newPg.Spec.Parameters.Encryption
		oldEncryption := &oldPg.Spec.Parameters.Encryption
		fieldPath := "spec.parameters.encryption.enabled"
		if err := validatePostgreSQLEncryptionChanges(newPg.GetName(), newEncryption, oldEncryption, fieldPath); err != nil {
			return nil, err
		}
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(pgGK, newPg.GetName(), allErrs)
	}

	return nil, nil
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

func (p *PostgreSQLWebhookHandler) checkGuaranteedAvailability(pg *vshnv1.VSHNPostgreSQL) field.ErrorList {
	allErrs := field.ErrorList{}
	if pg.Spec.Parameters.Service.ServiceLevel == "guaranteed" && pg.Spec.Parameters.Instances < 2 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.parameters.instances"),
			pg.Spec.Parameters.Instances,
			"PostgreSQL instances with service level Guaranteed Availability must have at least 2 replicas. Please set .spec.parameters.instances: [2,3]. Additional costs will apply, please refer to: https://products.vshn.ch/appcat/pricing.html",
		))
	}
	return allErrs
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

func validatePgConf(pg *vshnv1.VSHNPostgreSQL) field.ErrorList {
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
			allErrs = append(allErrs, field.Forbidden(field.NewPath(fmt.Sprintf("spec.parameters.service.postgresqlSettings[%s]", key)), "https://stackgres.io/doc/latest/api/responses/error/#postgres-blocklist"))
		}
	}

	return allErrs
}

func validateMajorVersionUpgrade(newPg *vshnv1.VSHNPostgreSQL, oldPg *vshnv1.VSHNPostgreSQL) (errList field.ErrorList) {

	newVersion, err := strconv.Atoi(newPg.Spec.Parameters.Service.MajorVersion)
	if err != nil {
		errList = append(errList, field.Invalid(
			field.NewPath("spec.parameters.service.majorVersion"),
			newPg.Spec.Parameters.Service.MajorVersion,
			fmt.Sprintf("invalid major version: %s", err.Error()),
		))
	}
	var oldVersion int
	if oldPg.Status.CurrentVersion == "" {
		oldVersion = newVersion
	} else {
		oldVersion, err = strconv.Atoi(oldPg.Status.CurrentVersion)
		if err != nil {
			errList = append(errList, field.Invalid(
				field.NewPath("status.currentVersion"),
				oldPg.Status.CurrentVersion,
				fmt.Sprintf("invalid major version: %s", err.Error()),
			))
		}
	}

	if newVersion != oldVersion {
		errList = append(errList, field.Invalid(
			field.NewPath("spec.parameters.service.majorVersion"),
			newPg.Spec.Parameters.Service.MajorVersion,
			"major version upgrade is not allowed.",
		))
		return errList
	}

	// Check if the upgrade is allowed
	if newVersion != oldVersion {
		if oldVersion != newVersion-1 {
			errList = append(errList, field.Forbidden(
				field.NewPath("spec.parameters.service.majorVersion"),
				"only one major version upgrade at a time is allowed",
			))
		}
		for _, e := range oldPg.Spec.Parameters.Service.Extensions {
			if e.Name == "timescaledb" || e.Name == "postgis" {
				errList = append(errList, field.Forbidden(
					field.NewPath("spec.parameters.service.majorVersion"),
					"major upgrades are not supported for instances with timescaledb or postgis extensions",
				))
			}
		}
		if newPg.Spec.Parameters.Instances > 1 {
			errList = append(errList, field.Forbidden(
				field.NewPath("spec.parameters.instances"),
				"major upgrades are not supported for HA instances",
			))
		}
	}
	return errList
}

func validatePostgreSQLEncryptionChanges(name string, newEncryption, oldEncryption *vshnv1.VSHNPostgreSQLEncryption, fieldPath string) error {
	// Check if encryption setting is being changed
	if newEncryption.Enabled != oldEncryption.Enabled {
		errList := field.ErrorList{}
		errList = append(errList, field.Forbidden(
			field.NewPath(fieldPath),
			"encryption setting cannot be changed after instance creation. It can only be set during initial creation.",
		))
		return apierrors.NewInvalid(pgGK, name, errList)
	}
	return nil
}
