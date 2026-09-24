package webhooks

import (
	"fmt"
	"strconv"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const stackgresBlocklistDetail = "https://stackgres.io/doc/latest/api/responses/error/#postgres-blocklist"

// stackgresBlocklist contains the PostgreSQL settings that StackGres manages itself.
var stackgresBlocklist = map[string]struct{}{
	"listen_addresses":      {},
	"port":                  {},
	"cluster_name":          {},
	"hot_standby":           {},
	"fsync":                 {},
	"full_page_writes":      {},
	"log_destination":       {},
	"logging_collector":     {},
	"max_replication_slots": {},
	"max_wal_senders":       {},
	"wal_keep_segments":     {},
	"wal_level":             {},
	"wal_log_hints":         {},
	"archive_mode":          {},
	"archive_command":       {},
}

var _ pgValidator = stackgresValidator{}

// stackgresValidator validates VSHNPostgreSQL instances backed by StackGres.
type stackgresValidator struct{}

func (stackgresValidator) validate(newPg, oldPg *vshnv1.VSHNPostgreSQL) field.ErrorList {
	allErrs := validatePgConf(newPg, stackgresBlocklist, stackgresBlocklistDetail)

	allErrs = append(allErrs, validateStackGresExtensions(newPg)...)

	if oldPg != nil {
		allErrs = append(allErrs, validateStackGresMajorVersion(newPg, oldPg)...)
	}

	return allErrs
}

// validateStackGresExtensions ensures that the CNPG only extension fields are not set.
func validateStackGresExtensions(pg *vshnv1.VSHNPostgreSQL) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, ext := range pg.Spec.Parameters.Service.Extensions {
		basePath := field.NewPath("spec", "parameters", "service", "extensions").Index(i)
		if ext.Image != "" {
			allErrs = append(allErrs, field.Forbidden(
				basePath.Child("image"),
				"image is only supported for CloudNativePG",
			))
		}
		if ext.ImagePullPolicy != "" {
			allErrs = append(allErrs, field.Forbidden(
				basePath.Child("imagePullPolicy"),
				"imagePullPolicy is only supported for CloudNativePG",
			))
		}
	}
	return allErrs
}

// validateStackGresMajorVersion blocks any major version change, StackGres supports neither upgrades nor downgrades.
func validateStackGresMajorVersion(newPg, oldPg *vshnv1.VSHNPostgreSQL) (errList field.ErrorList) {
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
		// CurrentVersion can be either major version ("15") or full version ("15.9")
		// Extract just the major version part
		currentVersion := oldPg.Status.CurrentVersion
		if idx := strings.Index(currentVersion, "."); idx > 0 {
			currentVersion = currentVersion[:idx]
		}
		oldVersion, err = strconv.Atoi(currentVersion)
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
