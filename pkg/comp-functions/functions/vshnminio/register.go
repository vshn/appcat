package vshnminio

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

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
			{
				Name:    "non-sla-prometheus-rules",
				Execute: common.GenerateNonSLAPromRules(&vshnv1.VSHNMinio{}),
			},
			{
				Name:    "securitycontext",
				Execute: SetSecurityContext,
			},
		},
	})
}
