package vshnkeycloak

import "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

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
		},
	})
}
