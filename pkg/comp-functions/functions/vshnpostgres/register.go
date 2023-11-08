package vshnpostgres

import (
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("postgresql", runtime.Service{
		Steps: []runtime.Step{
			{
				Name:    "url-connection-details",
				Execute: AddUrlToConnectionDetails,
			},
			{
				Name:    "user-alerting",
				Execute: AddUserAlerting,
			},
			{
				Name:    "restart",
				Execute: TransformRestart,
			},
			{
				Name:    "random-default-schedule",
				Execute: TransformSchedule,
			},
			{
				Name:    "encrypted-pvc-secret",
				Execute: AddPvcSecret,
			},
			{
				Name:    "maintenance-job",
				Execute: addSchedules,
			},
			{
				Name:    "mailgun-alerting",
				Execute: MailgunAlerting,
			},
			{
				Name:    "extensions",
				Execute: AddExtensions,
			},
			{
				Name:    "replication",
				Execute: ConfigureReplication,
			},
			{
				Name:    "load-balancer",
				Execute: AddLoadBalancerIPToConnectionDetails,
			},
			{
				Name:    "namespaceQuotas",
				Execute: common.AddInitialNamespaceQuotas("namespace-conditions"),
			},
		},
	})
}
