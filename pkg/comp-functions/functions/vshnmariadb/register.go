package vshnmariadb

import "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

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
		},
	})
}
