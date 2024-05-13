package vshnmariadb

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("mariadb", runtime.Service{
		Steps: []runtime.Step{

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
				Execute: common.MailgunAlerting(&vshnv1.VSHNMariaDB{}),
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting(&vshnv1.VSHNMariaDB{}),
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: common.GenerateNonSLAPromRules(&vshnv1.VSHNMariaDB{}, common.AlertsEnabled{All: true}),
			},
		},
	})
}
