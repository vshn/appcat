package vshnpostgrescnpg

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var pgAlertsBuilder = nonsla.NewAlertSetBuilder("postgres").
	AddAll().
	AddCustomServiceRule("maxconnections", maxConnectionsAlert).
	AddCustomServiceRule("longrunningtx", longRunningTransactionAlert).
	AddCustomServiceRule("backendwaiting", backendsWaitingAlert).
	AddCustomServiceRule("deadlocks", deadlockConflictsAlert).
	AddCustomServiceRule("highconnectionswarning", highConnectionsWarningAlert).
	AddCustomServiceRule("lowdiskwarning", lowDiskSpaceWarningAlert).
	AddCustomServiceRule("lowdiskcritical", lowDiskSpaceCriticalAlert)

var (
	// pgAlertsHibernated is used for suspended instances (instances: 0). The cluster is
	// hibernated, no pods run and cnpg_collector_up is absent, so CNPGClusterOffline
	// would always fire.
	// It must be declared before pgAlerts: both use the same builder, and pgAlerts adds
	// CNPGClusterOffline to it.
	pgAlertsHibernated = pgAlertsBuilder.GetAlerts()
	pgAlerts           = pgAlertsBuilder.AddCustomServiceRule("cnpgclusteroffline", clusterOfflineAlert).GetAlerts()
)

// generateNonSLAPromRules renders the non-SLA rules, without CNPGClusterOffline for
// hibernated instances. It uses the same check as the hibernation annotation in deploy.go.
func generateNonSLAPromRules(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	alerts := pgAlerts
	if comp.GetInstances() == 0 {
		alerts = pgAlertsHibernated
	}
	return nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNPostgreSQL](alerts)(ctx, comp, svc)
}

func init() {
	runtime.RegisterService[*vshnv1.VSHNPostgreSQL]("postgresqlcnpg", runtime.Service[*vshnv1.VSHNPostgreSQL]{
		Steps: []runtime.Step[*vshnv1.VSHNPostgreSQL]{
			{
				Name:    "deploy",
				Execute: DeployPostgreSQL,
			},
			{
				Name:    "netpol",
				Execute: createCnpgNetworkPolicy,
			},
			{
				Name:    "tcpgateway",
				Execute: ConfigureTCPGateway,
			},
			{
				Name:    "connection-details",
				Execute: AddConnectionSecrets,
			},
			{
				Name:    "user-management",
				Execute: UserManagement,
			},
			{
				Name:    "maintenance",
				Execute: addSchedules,
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "additional-resources",
				Execute: common.AddAdditionalResources[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "encrypted-pvc-secret",
				Execute: AddPvcSecret,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: generateNonSLAPromRules,
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
		},
	})
}
