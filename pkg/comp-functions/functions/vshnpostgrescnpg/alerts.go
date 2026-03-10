package vshnpostgrescnpg

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/nonsla"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func longRunningTransactionAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "PostgreSQLLongRunningTransaction",
		Annotations: map[string]string{
			"description": "A transaction on {{ $labels.pod }} in namespace " + namespace + " has been running for over 300 seconds.\n  Please investigate and terminate long-running transactions if necessary.",
			"runbook_url": "https://kb.vshn.ch/app-catalog/service/postgresql/runbooks/user/alert-postgresqllongrunningtransaction.html",
			"summary":     "PostgreSQL long running transaction detected",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: `cnpg_backends_max_tx_duration_seconds{namespace="` + namespace + `"} > 300`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityWarning,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func backendsWaitingAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "PostgreSQLBackendsWaiting",
		Annotations: map[string]string{
			"description": "The number of backends waiting on locks on {{ $labels.pod }} in namespace " + namespace + " is currently above 300.\n  Please investigate lock contention in your application.",
			"runbook_url": "https://kb.vshn.ch/app-catalog/service/postgresql/runbooks/user/alert-postgresqlbackendswaiting.html",
			"summary":     "PostgreSQL backends waiting on locks",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: `cnpg_backends_waiting_total{namespace="` + namespace + `"} > 300`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityWarning,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func deadlockConflictsAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "PostgreSQLDeadlockConflicts",
		Annotations: map[string]string{
			"description": "More than 10 deadlocks have been detected on {{ $labels.datname }} in namespace " + namespace + " in the last 5 minutes.\n  Please review your application's transaction handling to eliminate deadlocks.",
			"runbook_url": "https://kb.vshn.ch/app-catalog/service/postgresql/runbooks/user/alert-postgresqldeadlockconflicts.html",
			"summary":     "PostgreSQL deadlock conflicts detected",
		},
		Expr: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: `cnpg_pg_stat_database_deadlocks{namespace="` + namespace + `"} > 10`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityWarning,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func highConnectionsWarningAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "CNPGClusterHighConnectionsWarning",
		Annotations: map[string]string{
			"description": "CNPG Instance is approaching the maximum number of connections in namespace " + namespace + ".",
			"runbook_url": "https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/docs/runbooks/CNPGClusterHighConnectionsWarning.md",
			"summary":     "CNPG Instance is approaching the maximum number of connections",
		},
		Expr: intstr.IntOrString{
			Type: intstr.String,
			StrVal: `sum by (pod) (cnpg_backends_total{namespace="` + namespace + `"})
  / max by (pod) (cnpg_pg_settings_setting{name="max_connections", namespace="` + namespace + `"})
  * 100 > 80`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityWarning,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func lowDiskSpaceWarningAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "CNPGClusterLowDiskSpaceWarning",
		Annotations: map[string]string{
			"description": "CNPG Cluster in namespace " + namespace + " is running low on disk space (>70% used).",
			"runbook_url": "https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/docs/runbooks/CNPGClusterLowDiskSpaceWarning.md",
			"summary":     "CNPG Cluster is running low on disk space",
		},
		Expr: intstr.IntOrString{
			Type: intstr.String,
			StrVal: `max(max by(persistentvolumeclaim) (
    1 - kubelet_volume_stats_available_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*"}
      / kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*"}
  )) > 0.7
OR
  max(max by(persistentvolumeclaim) (
    1 - kubelet_volume_stats_available_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-wal"}
      / kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-wal"}
  )) > 0.7
OR
  max(
    sum by (namespace,persistentvolumeclaim) (kubelet_volume_stats_used_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-tbs.*"})
    / sum by (namespace,persistentvolumeclaim) (kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-tbs.*"})
    * on(namespace, persistentvolumeclaim) group_left(volume)
      kube_pod_spec_volumes_persistentvolumeclaims_info{namespace="` + namespace + `", pod=~"postgresql-.*"}
  ) > 0.7`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityWarning,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func lowDiskSpaceCriticalAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "CNPGClusterLowDiskSpaceCritical",
		Annotations: map[string]string{
			"description": "CNPG Cluster in namespace " + namespace + " is critically low on disk space (>90% used).",
			"runbook_url": "https://github.com/cloudnative-pg/charts/blob/main/charts/cluster/docs/runbooks/CNPGClusterLowDiskSpaceCritical.md",
			"summary":     "CNPG Cluster is critically low on disk space",
		},
		Expr: intstr.IntOrString{
			Type: intstr.String,
			StrVal: `max(max by(persistentvolumeclaim) (
    1 - kubelet_volume_stats_available_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*"}
      / kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*"}
  )) > 0.9
OR
  max(max by(persistentvolumeclaim) (
    1 - kubelet_volume_stats_available_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-wal"}
      / kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-wal"}
  )) > 0.9
OR
  max(
    sum by (namespace,persistentvolumeclaim) (kubelet_volume_stats_used_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-tbs.*"})
    / sum by (namespace,persistentvolumeclaim) (kubelet_volume_stats_capacity_bytes{namespace="` + namespace + `", persistentvolumeclaim=~"postgresql-.*-tbs.*"})
    * on(namespace, persistentvolumeclaim) group_left(volume)
      kube_pod_spec_volumes_persistentvolumeclaims_info{namespace="` + namespace + `", pod=~"postgresql-.*"}
  ) > 0.9`,
		},
		For: nonsla.FiveMinuteInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityCritical,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}

func maxConnectionsAlert(service, namespace string) promv1.Rule {
	return promv1.Rule{
		Alert: "PostgreSQLConnectionsCritical",
		Annotations: map[string]string{
			"description": "The connections to {{ $labels.pod }} have been over 90% of the configured connections for 2 hours.\n  Please reduce the load of this instance.",
			"runbook_url": "https://kb.vshn.ch/app-catalog/how-tos/appcat/vshn/postgres/PostgreSQLConnectionsCritical.html",
			"summary":     "Connection usage critical",
		},
		Expr: intstr.IntOrString{
			Type: intstr.String,
			StrVal: `sum(cnpg_backends_total{namespace="` + namespace + `", application_name="cnpg_metrics_exporter"}) by (pod)
  > 0.9 * max(cnpg_pg_settings_setting{name="max_connections", namespace="` + namespace + `"}) by (pod)`,
		},
		For: nonsla.TwoHourInterval,
		Labels: map[string]string{
			"severity": nonsla.SeverityCritical,
			"syn_team": nonsla.SynTeam,
			"syn":      "true",
		},
	}
}
