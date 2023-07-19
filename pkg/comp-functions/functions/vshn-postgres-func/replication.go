package vshnpostgres

import (
	"context"

	stackgresv1 "github.com/vshn/appcat/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"k8s.io/utils/pointer"
)

const replicationModeAsync = "async"

// ConfigureReplication configures the stackgres replication based on the claim
func ConfigureReplication(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}
	cluster := &stackgresv1.SGCluster{}
	err = iof.Desired.GetFromObject(ctx, cluster, "cluster")
	if err != nil {
		return runtime.NewFatalErr(ctx, "not able to get cluster", err)
	}

	cluster = configureReplication(ctx, comp, cluster)

	err = iof.Desired.PutIntoObject(ctx, cluster, "cluster")
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot save cluster to functionIO", err)
	}
	return runtime.NewNormal()
}

func configureReplication(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, cluster *stackgresv1.SGCluster) *stackgresv1.SGCluster {
	cluster.Spec.Replication = &stackgresv1.SGClusterSpecReplication{
		Mode:          pointer.String(replicationModeAsync),
		SyncInstances: pointer.Int(1),
	}
	cluster.Spec.Instances = comp.Spec.Parameters.Instances
	if comp.Spec.Parameters.Instances > 1 && comp.Spec.Parameters.Replication.Mode != replicationModeAsync && comp.Spec.Parameters.Replication.Mode != "" {
		cluster.Spec.Replication.Mode = pointer.String(comp.Spec.Parameters.Replication.Mode)
		cluster.Spec.Replication.SyncInstances = pointer.Int(comp.Spec.Parameters.Instances - 1)
	}
	return cluster
}
