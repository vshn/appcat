package vshnforgejo

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("forgejo", runtime.Service[*vshnv1.VSHNForgejo]{
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
		},
	})
}
