package vshnmariadb

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNMariaDB]("mariadb", runtime.Service[*vshnv1.VSHNMariaDB]{
		Steps: []runtime.Step[*vshnv1.VSHNMariaDB]{

			{
				Name:    "deploy",
				Execute: DeployMariadb,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "backup",
				Execute: AddBackupMariadb,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNMariaDB],
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNMariaDB],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNMariaDB](nonsla.NewAlertSetBuilder("mariadb").AddAll().GetAlerts()),
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
			{
				Name:    "user-management",
				Execute: UserManagement,
			},
			{
				Name:    "proxySQL",
				Execute: AddProxySQL,
			},
		},
	})
}
