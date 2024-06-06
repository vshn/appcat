package vshnforgejo

import (
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("forgejo", runtime.Service{
		Steps: []runtime.Step{

			{
				Name:    "deploy",
				Execute: DeployForgejo,
			},
		},
	})
}
