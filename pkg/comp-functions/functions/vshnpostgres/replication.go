package vshnpostgres

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/utils/pointer"
)

const replicationModeAsync = "async"

// ConfigureReplication configures the stackgres replication based on the claim
func ConfigureReplication(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}
	cluster := &stackgresv1.SGCluster{}
	err = svc.GetDesiredKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("not able to get cluster: %w", err))
	}

	cluster = configureReplication(ctx, comp, cluster)

	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot save cluster to functionIO: %w", err))
	}
	return nil
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
