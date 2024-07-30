package cloudscalebucket

import "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

func init() {
	runtime.RegisterService("cloudscalebucket", runtime.Service{
		Steps: []runtime.Step{
			{
				Name:    "provision-bucket",
				Execute: ProvisionCloudscalebucket,
			},
		},
	})
}
