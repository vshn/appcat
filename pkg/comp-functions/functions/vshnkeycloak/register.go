package vshnkeycloak

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("keycloak", runtime.Service{
		Steps: []runtime.Step{

			{
				Name:    "deploy",
				Execute: DeployKeycloak,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "ingress",
				Execute: AddIngress,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting(&vshnv1.VSHNKeycloak{}),
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting(&vshnv1.VSHNKeycloak{}),
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules(&vshnv1.VSHNKeycloak{}, nonsla.NewAlertSetBuilder("keycloak").AddMemory().GetAlerts()),
			},
		},
	})
}
