package vshnforgejo

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNForgejo]("forgejo", runtime.Service[*vshnv1.VSHNForgejo]{
		Steps: []runtime.Step[*vshnv1.VSHNForgejo]{

			{
				Name:    "deploy",
				Execute: DeployForgejo,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "backup",
				Execute: AddBackup,
			},
			{
				Name:    "ssh",
				Execute: ConfigureSSHAccess,
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNForgejo],
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNForgejo],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNForgejo](nonsla.NewAlertSetBuilder("forgejo").AddAll().GetAlerts()),
			},
			{
				Name:    "additional-resources",
				Execute: common.AddAdditionalResources[*vshnv1.VSHNForgejo],
			},
		},
	})
}
