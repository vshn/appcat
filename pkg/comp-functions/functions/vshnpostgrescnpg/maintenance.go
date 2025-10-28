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
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	common.SetRandomMaintenanceSchedule(comp)
	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	additionalVars := append(extraEnvVars, []corev1.EnvVar{
		{
			Name:  "VACUUM_ENABLED",
			Value: strconv.FormatBool(comp.Spec.Parameters.Service.VacuumEnabled),
		},
	}...)

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithRole(maintRolename).
		WithAdditionalClusterRoleBinding(fmt.Sprintf("%s:%s", maintRolename, comp.GetName())).
		WithPolicyRules(policyRules).
		WithExtraEnvs(additionalVars...).
		Run(ctx)
}
