package exoscalebucket

import (
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*appcatv1.ObjectBucket]("exoscalebucket", runtime.Service[*appcatv1.ObjectBucket]{
		Steps: []runtime.Step[*appcatv1.ObjectBucket]{
			{
				Name:    "provision-bucket",
				Execute: ProvisionExoscalebucket,
			},
		},
	})
}
