package vshnnextcloud

import (
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("nextcloud", runtime.Service{
		Steps: []runtime.Step{

			{
				Name:    "deploy",
				Execute: DeployNextcloud,
			},
			{
				Name:    "ingress",
				Execute: AddIngress,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
		},
	})
}
