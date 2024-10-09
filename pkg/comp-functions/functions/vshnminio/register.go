package vshnminio

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNMinio]("minio", runtime.Service[*vshnv1.VSHNMinio]{
		Steps: []runtime.Step[*vshnv1.VSHNMinio]{

			{
				Name:    "deploy",
				Execute: DeployMinio,
			},
			{
				Name:    "deploy-providerconfig",
				Execute: DeployMinioProviderConfig,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNMinio](nonsla.NewAlertSetBuilder("minio", "minio").AddAll().GetAlerts()),
			},
			{
				Name:    "securitycontext",
				Execute: SetSecurityContext,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNMinio],
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNMinio],
			},
			{
				Name:    "pdb",
				Execute: common.AddPDBSettings[*vshnv1.VSHNMinio],
			},
			{
				Name:    "billing",
				Execute: AddServiceBillingLabel,
			},
		},
	})
}
