package vshnpostgres

import (
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/utils/ptr"
)

const replicationModeAsync = "async"

func configureReplication(comp *vshnv1.VSHNPostgreSQL, cluster *stackgresv1.SGCluster) *stackgresv1.SGCluster {
	cluster.Spec.Replication = &stackgresv1.SGClusterSpecReplication{
		Mode:          ptr.To(replicationModeAsync),
		SyncInstances: ptr.To(1),
	}
	cluster.Spec.Instances = comp.Spec.Parameters.Instances
	if comp.Spec.Parameters.Instances > 1 && comp.Spec.Parameters.Replication.Mode != replicationModeAsync && comp.Spec.Parameters.Replication.Mode != "" {
		cluster.Spec.Replication.Mode = ptr.To(comp.Spec.Parameters.Replication.Mode)
		cluster.Spec.Replication.SyncInstances = ptr.To(comp.Spec.Parameters.Instances - 1)
	}
	return cluster
}
