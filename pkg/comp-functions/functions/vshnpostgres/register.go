package vshnpostgres

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var pgAlerts = nonsla.NewAlertSetBuilder("patroni").AddAll().AddCustomServiceRule("maxconnections", maxConnectionsAlert).GetAlerts()

func init() {
	runtime.RegisterService[*vshnv1.VSHNPostgreSQL]("postgresql", runtime.Service[*vshnv1.VSHNPostgreSQL]{
		Steps: []runtime.Step[*vshnv1.VSHNPostgreSQL]{
			{
				Name:    "deploy",
				Execute: DeployPostgreSQL,
			},
			{
				Name:    "connectiondetails",
				Execute: AddConnectionDetails,
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "restart",
				Execute: TransformRestart,
			},
			{
				Name:    "random-default-schedule",
				Execute: TransformSchedule,
			},
			{
				Name:    "encrypted-pvc-secret",
				Execute: AddPvcSecret,
			},
			{
				Name:    "maintenance-job",
				Execute: addSchedules,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "extensions",
				Execute: AddExtensions,
			},
			{
				Name:    "load-balancer",
				Execute: AddPrimaryService,
			},
			{
				Name:    "delay-cluster-deployment",
				Execute: DelayClusterDeployment,
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNPostgreSQL](pgAlerts),
			},
			{
				Name:    "pgbouncer-settings",
				Execute: addPGBouncerSettings,
			},
			{
				Name:    "ensure-objectbucket-labels",
				Execute: EnsureObjectBucketLabels,
			},
			{
				Name:    "user-management",
				Execute: UserManagement,
			},
			{
				Name:    "pdb",
				Execute: common.AddPDBSettings[*vshnv1.VSHNPostgreSQL],
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
		},
	})
}
