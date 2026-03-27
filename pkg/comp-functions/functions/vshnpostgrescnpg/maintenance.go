package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	service       = "postgresql_cnpg"
	maintRolename = "crossplane:appcat:job:postgres:maintenance"
	policyRules   = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"postgresql.cnpg.io",
			},
			Resources: []string{
				"imagecatalogs",
			},
			Verbs: []string{
				"delete",
				"create",
				"update",
				"patch",
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"postgresql.cnpg.io",
			},
			Resources: []string{
				"clusters",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"postgresql.cnpg.io",
			},
			Resources: []string{
				"backups",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
	extraEnvVars = []corev1.EnvVar{}
)

// addSchedules will add a job to do the maintenance for the instance
func addSchedules(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	// Try desired first to preserve status set by deploy step (including CurrentVersion),
	// fall back to observed if desired is not available.
	if err := svc.GetDesiredComposite(comp); err != nil {
		if err := svc.GetObservedComposite(comp); err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
	}

	common.SetRandomMaintenanceSchedule(comp)
	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		svc.Log.Error(err, "cannot set composite status")
	}

	// Disable vacuum for suspended instances
	vacuumEnabled := comp.Spec.Parameters.Service.VacuumEnabled && comp.Spec.Parameters.Instances != 0

	additionalVars := append(extraEnvVars, []corev1.EnvVar{
		{
			Name:  "VACUUM_ENABLED",
			Value: strconv.FormatBool(vacuumEnabled),
		},
	}...)

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithRole(maintRolename).
		WithAdditionalClusterRoleBinding(fmt.Sprintf("%s:%s", maintRolename, comp.GetName())).
		WithPolicyRules(policyRules).
		WithExtraEnvs(additionalVars...).
		Run(ctx)
}
