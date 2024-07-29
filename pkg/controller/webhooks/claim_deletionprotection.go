package webhooks

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func GetClaimDeletionProtection(security *vshnv1.Security, allErrs field.ErrorList) field.ErrorList {
	if security.DeletionProtection {
		allErrs = append(allErrs, &field.Error{
			Field:  "spec.parameters.security.deletionProtection",
			Detail: "DeletionProtection is enabled. To delete the instance, disable the deletionProtection in spec.parameters.security.deletionProtection",
			Type:   field.ErrorTypeForbidden,
		})
	}
	return allErrs
}
