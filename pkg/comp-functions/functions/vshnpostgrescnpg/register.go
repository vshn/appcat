package vshnpostgrescnpg

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var pgAlerts = nonsla.NewAlertSetBuilder("patroni").AddAll().AddCustomServiceRule("maxconnections", maxConnectionsAlert).GetAlerts()

func init() {
	runtime.RegisterService[*vshnv1.VSHNPostgreSQLCNPG]("postgresqlcnpg", runtime.Service[*vshnv1.VSHNPostgreSQLCNPG]{
		Steps: []runtime.Step[*vshnv1.VSHNPostgreSQLCNPG]{
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
				Execute: common.AddUserAlerting[*vshnv1.VSHNPostgreSQLCNPG],
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
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNPostgreSQLCNPG],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNPostgreSQLCNPG](pgAlerts),
			},
			{
				Name:    "ensure-objectbucket-labels",
				Execute: EnsureObjectBucketLabels,
			},
			{
				Name:    "pdb",
				Execute: common.AddPDBSettings[*vshnv1.VSHNPostgreSQLCNPG],
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
		},
	})
}
