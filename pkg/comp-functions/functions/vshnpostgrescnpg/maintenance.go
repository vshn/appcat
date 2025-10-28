package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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
	}

	extraEnvVars = []corev1.EnvVar{}
)

// addSchedules will add a job to do the maintenance for the instance
func addSchedules(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	cd, err := getConnectionDetails(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set up maintenance yet: %v", err.Error()))
	}

	uri := string(cd["POSTGRESQL_URL"])
	if strings.HasSuffix(uri, "*") {
		uri = fmt.Sprintf("%s%s", strings.TrimSuffix(uri, "*"), string(cd["POSTGRESQL_DB"]))
	}

	additionalVars := append(extraEnvVars, []corev1.EnvVar{
		{
			Name:  "VACUUM_ENABLED",
			Value: strconv.FormatBool(comp.Spec.Parameters.Service.VacuumEnabled),
		},
		{
			Name:  "PSQL_URI",
			Value: uri,
		},
	}...)

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithRole(maintRolename).
		WithAdditionalClusterRoleBinding(fmt.Sprintf("%s:%s", maintRolename, comp.GetName())).
		WithPolicyRules(policyRules).
		WithExtraEnvs(additionalVars...).
		//WithExtraResources(createMaintenanceSecret(instanceNamespace, sgNamespace, comp.GetName()+"-maintenance-secret", comp.GetName())).
		Run(ctx)
}
