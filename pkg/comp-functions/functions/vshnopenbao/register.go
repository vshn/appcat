package vshnopenbao

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNOpenBao]("openbao", runtime.Service[*vshnv1.VSHNOpenBao]{
		Steps: []runtime.Step[*vshnv1.VSHNOpenBao]{
			{
				Name:    "deploy",
				Execute: DeployOpenBao,
			},
		},
	})
}
