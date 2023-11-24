package miniobucket

import "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

func init() {
	runtime.RegisterService("miniobucket", runtime.Service{
		Steps: []runtime.Step{
			{
				Name:    "provision-bucket",
				Execute: ProvisionMiniobucket,
			},
		},
	})
}
