package vshnminio

import "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

func init() {
	runtime.RegisterService("minio", runtime.Service{
		Steps: []runtime.Step{

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
		},
	})
}
