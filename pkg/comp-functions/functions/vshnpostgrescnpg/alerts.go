package vshnpostgrescnpg

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func maxConnectionsAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "PostgreSQLConnectionsCritical",
		Annotations: map[string]string{
			"description": "The connections to {{ $labels.pod }} have been over 90% of the configured connections for 2 hours.\n  Please reduce the load of this instance.",
			"runbook_url": "https://kb.vshn.ch/app-catalog/how-tos/appcat/vshn/postgres/PostgreSQLConnectionsCritical.html",
			"summary":     "Connection usage critical",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: `sum(cnpg_backends_total{namespace="` + namespace + `", application_name="cnpg_metrics_exporter"}) by (pod) > 0.9 * max(cnpg_pg_settings_setting{name="max_connections", namespace="` + namespace + `"}) by (pod)`,
		},
		For: nonsla.TwoHourInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityCritical,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}
