package webhooks

import (
	"fmt"
	"strconv"

	"github.com/blang/semver/v4"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const cnpgBlocklistDetail = "https://cloudnative-pg.io/docs/current/postgresql_conf/#fixed-parameters"

// cnpgBlocklist mirrors the fixed parameters of CloudNativePG, which the operator
// controls exclusively and rejects in its own webhook.
// https://cloudnative-pg.io/docs/1.30/postgresql_conf#fixed-parameters
var cnpgBlocklist = map[string]struct{}{
	"allow_alter_system":                     {},
	"allow_system_table_mods":                {},
	"archive_cleanup_command":                {},
	"archive_command":                        {},
	"archive_mode":                           {},
	"bonjour":                                {},
	"bonjour_name":                           {},
	"cluster_name":                           {},
	"config_file":                            {},
	"data_directory":                         {},
	"data_sync_retry":                        {},
	"event_source":                           {},
	"external_pid_file":                      {},
	"hba_file":                               {},
	"hot_standby":                            {},
	"ident_file":                             {},
	"jit_provider":                           {},
	"listen_addresses":                       {},
	"log_destination":                        {},
	"log_directory":                          {},
	"log_file_mode":                          {},
	"log_filename":                           {},
	"log_rotation_age":                       {},
	"log_rotation_size":                      {},
	"log_truncate_on_rotation":               {},
	"logging_collector":                      {},
	"port":                                   {},
	"primary_conninfo":                       {},
	"primary_slot_name":                      {},
	"promote_trigger_file":                   {},
	"recovery_end_command":                   {},
	"recovery_min_apply_delay":               {},
	"recovery_target":                        {},
	"recovery_target_action":                 {},
	"recovery_target_inclusive":              {},
	"recovery_target_lsn":                    {},
	"recovery_target_name":                   {},
	"recovery_target_time":                   {},
	"recovery_target_timeline":               {},
	"recovery_target_xid":                    {},
	"restart_after_crash":                    {},
	"restore_command":                        {},
	"shared_preload_libraries":               {},
	"ssl":                                    {},
	"ssl_ca_file":                            {},
	"ssl_cert_file":                          {},
	"ssl_crl_file":                           {},
	"ssl_dh_params_file":                     {},
	"ssl_ecdh_curve":                         {},
	"ssl_key_file":                           {},
	"ssl_passphrase_command":                 {},
	"ssl_passphrase_command_supports_reload": {},
	"ssl_prefer_server_ciphers":              {},
	"stats_temp_directory":                   {},
	"synchronous_standby_names":              {},
	"syslog_facility":                        {},
	"syslog_ident":                           {},
	"syslog_sequence_numbers":                {},
	"syslog_split_messages":                  {},
	"unix_socket_directories":                {},
	"unix_socket_group":                      {},
	"unix_socket_permissions":                {},
}

var _ pgValidator = cnpgValidator{}

// cnpgValidator validates VSHNPostgreSQL instances backed by CloudNativePG.
type cnpgValidator struct{}

func (cnpgValidator) validate(newPg, oldPg *vshnv1.VSHNPostgreSQL) field.ErrorList {
	allErrs := validatePgConf(newPg, cnpgBlocklist, cnpgBlocklistDetail)

	allErrs = append(allErrs, validateCNPGExtensions(newPg)...)

	if oldPg != nil {
		if err := validateNoDowngrade(
			oldPg.Status.CurrentVersion,
			newPg.Spec.Parameters.Service.MajorVersion,
			field.NewPath("spec", "parameters", "service", "majorVersion"),
		); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}

// validateCNPGExtensions ensures that image and imagePullPolicy are only used from
// cnpgExtensionMinMajorVersion onwards.
func validateCNPGExtensions(pg *vshnv1.VSHNPostgreSQL) field.ErrorList {
	allErrs := field.ErrorList{}
	if isMajorVersionAtLeast(pg.Spec.Parameters.Service.MajorVersion, cnpgExtensionMinMajorVersion) {
		return allErrs
	}

	for i, ext := range pg.Spec.Parameters.Service.Extensions {
		basePath := field.NewPath("spec", "parameters", "service", "extensions").Index(i)
		if ext.Image != "" {
			allErrs = append(allErrs, field.Forbidden(
				basePath.Child("image"),
				fmt.Sprintf("image is only supported for PostgreSQL %s and above", cnpgExtensionMinMajorVersion),
			))
		}
		if ext.ImagePullPolicy != "" {
			allErrs = append(allErrs, field.Forbidden(
				basePath.Child("imagePullPolicy"),
				fmt.Sprintf("imagePullPolicy is only supported for PostgreSQL %s and above", cnpgExtensionMinMajorVersion),
			))
		}
	}
	return allErrs
}

// isMajorVersionAtLeast returns true if version >= minVersion (both as major version strings like "18").
// Returns false if either value cannot be parsed.
func isMajorVersionAtLeast(version, minVersion string) bool {
	v, err := strconv.Atoi(version)
	if err != nil {
		return false
	}
	min, err := strconv.Atoi(minVersion)
	if err != nil {
		return false
	}
	return v >= min
}

// validateNoDowngrade returns an error if newVersion is lower than oldVersion.
// Both versions are parsed tolerantly, so plain major versions ("15"), semver ("15.9"), and full versions ("15.9.1") are all accepted.
// If either version is empty or unparseable as an old version, the check is skipped.
func validateNoDowngrade(oldVersion, newVersion string, path *field.Path) *field.Error {
	if oldVersion == "" || newVersion == "" {
		return nil
	}
	oldV, err := semver.ParseTolerant(oldVersion)
	if err != nil {
		return nil
	}
	newV, err := semver.ParseTolerant(newVersion)
	if err != nil {
		return field.Invalid(path, newVersion, fmt.Sprintf("invalid version %q", newVersion))
	}
	if newV.LT(oldV) {
		return field.Invalid(path, newVersion, fmt.Sprintf("downgrading from %q to %q is not supported", oldVersion, newVersion))
	}
	return nil
}
