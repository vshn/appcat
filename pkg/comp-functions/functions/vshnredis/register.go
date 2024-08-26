package vshnredis

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNRedis]("redis", runtime.Service[*vshnv1.VSHNRedis]{
		Steps: []runtime.Step[*vshnv1.VSHNRedis]{
			{
				Name:    "deploy",
				Execute: DeployRedis,
			},
			{
				Name:    "manage-release",
				Execute: ManageRelease,
			},
			{
				Name:    "backup",
				Execute: AddBackup,
			},
			{
				Name:    "restore",
				Execute: RestoreBackup,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "resizePVC",
				Execute: ResizePVCs,
			},
			{
				Name:    "redis_url",
				Execute: AddUrlToConnectionDetails,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNRedis],
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNRedis],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNRedis](nonsla.NewAlertSetBuilder("redis", "redis").AddAll().GetAlerts()),
			},
		},
	})
}
