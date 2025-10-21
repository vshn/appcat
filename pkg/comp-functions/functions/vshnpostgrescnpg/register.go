package vshnpostgrescnpg

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var pgAlerts = nonsla.NewAlertSetBuilder("patroni").AddAll().AddCustomServiceRule("maxconnections", maxConnectionsAlert).GetAlerts()

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
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNPostgreSQL],
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
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNPostgreSQL](pgAlerts),
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
		},
	})
}
