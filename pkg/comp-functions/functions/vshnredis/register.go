package vshnredis

import (
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("redis", runtime.Service{
		Steps: []runtime.Step{
			{
				Name:    "manage-release",
				Execute: ManageRelease,
			},
			{
				Name:    "backup",
				Execute: AddBackup,
			},
			{
				Name:    "restore",
				Execute: RestoreBackup,
			},
			{
				Name:    "maintenance",
				Execute: AddMaintenanceJob,
			},
			{
				Name:    "resizePVC",
				Execute: ResizePVCs,
			},
			{
				Name:    "namespaceQuotas",
				Execute: common.AddInitialNamespaceQuotas("namespace-conditions"),
			},
			{
				Name:    "redis_url",
				Execute: AddUrlToConnectionDetails,
			},
		},
	})
}
