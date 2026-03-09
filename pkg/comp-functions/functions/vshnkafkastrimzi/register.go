package vshnkafkastrimzi

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNKafkaStrimzi]("kafkastrimzi", runtime.Service[*vshnv1.VSHNKafkaStrimzi]{
		Steps: []runtime.Step[*vshnv1.VSHNKafkaStrimzi]{
			{
				Name:    "deploy",
				Execute: DeployKafkaStrimzi,
			},
			// {
			// 	Name:    "netpol",
			// 	Execute: createCnpgNetworkPolicy,
			// },
			// {
			// 	Name:    "connection-details",
			// 	Execute: AddConnectionSecrets,
			// },
			// {
			// 	Name:    "maintenance",
			// 	Execute: addSchedules,
			// },
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting[*vshnv1.VSHNKafkaStrimzi],
			},
			// {
			// 	Name:    "encrypted-pvc-secret",
			// 	Execute: AddPvcSecret,
			// },
			{
				Name:    "mailgun-alerting",
				Execute: common.MailgunAlerting[*vshnv1.VSHNKafkaStrimzi],
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: nonsla.GenerateNonSLAPromRules[*vshnv1.VSHNKafkaStrimzi](nonsla.NewAlertSetBuilder("kafkastrimzi").AddMemory().GetAlerts()),
			},
			{
				Name:    "billing",
				Execute: AddBilling,
			},
		},
	})
}
