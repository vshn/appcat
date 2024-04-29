package vshnpostgres

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("postgresql", runtime.Service{
		Steps: []runtime.Step{
			{
				Name:    "deploy",
				Execute: DeployPostgreSQL,
			},
			{
				Name:    "connectiondetails",
				Execute: AddConnectionDetails,
			},
			{
				Name:    "user-alerting",
				Execute: common.AddUserAlerting(&vshnv1.VSHNPostgreSQL{}),
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
				Execute: common.MailgunAlerting(&vshnv1.VSHNPostgreSQL{}),
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
			{
				Name:    "delay-cluster-deployment",
				Execute: DelayClusterDeployment,
			},
			{
				Name:    "non-sla-prometheus-rules",
				Execute: common.GenerateNonSLAPromRules(&vshnv1.VSHNPostgreSQL{}),
			},
			{
				Name:    "pgbouncer-settings",
				Execute: addPGBouncerSettings,
			},
			{
				Name:    "pg-exporter-Workaround",
				Execute: PgExporterConfig,
			},
			{
				Name:    "ensure-objectbucket-labels",
				Execute: EnsureObjectBucketLabels,
			},
		},
	})
}
