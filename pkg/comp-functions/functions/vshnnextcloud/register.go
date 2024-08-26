package vshnnextcloud

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNNextcloud]("nextcloud", runtime.Service[*vshnv1.VSHNNextcloud]{
		Steps: []runtime.Step[*vshnv1.VSHNNextcloud]{

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
			{
				Name:    "backup",
				Execute: AddBackup,
			},
		},
	})
}
