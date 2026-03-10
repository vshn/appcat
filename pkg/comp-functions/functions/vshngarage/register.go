package vshngarage

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNGarage]("garage", runtime.Service[*vshnv1.VSHNGarage]{
		Steps: []runtime.Step[*vshnv1.VSHNGarage]{
			{
				Name:    "deploy",
				Execute: DeployGarage,
			},
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNGarage],
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNGarage],
			},
			{
				Name:    "pdb",
				Execute: common.AddPDBSettings[*vshnv1.VSHNGarage],
			},
		},
	})
}
