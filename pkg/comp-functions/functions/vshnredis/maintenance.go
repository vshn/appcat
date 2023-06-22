package vshnredis

import (
	"context"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	service         = "redis"
	maintRolename   = "crossplane:appcat:job:redis:maintenance"
	maintSecretName = "maintenancesecret"

	policyRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"helm.crossplane.io",
			},
			Resources: []string{
				"releases",
			},
			Verbs: []string{
				"update",
				"get",
				"list",
				"watch",
			},
		},
	}
)

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := &vshnv1.VSHNRedis{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.Spec.Parameters.Maintenance

	return maintenance.New(comp, iof, schedule, policyRules, instanceNamespace, maintRolename, service).
		Run(ctx)
}
