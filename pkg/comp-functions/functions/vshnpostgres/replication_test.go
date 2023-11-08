package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func Test_ConfigureReplication(t *testing.T) {
	ctx := context.TODO()
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/replication/01-GivenMulitInstance.yaml")

	res := ConfigureReplication(ctx, svc)
	assert.Nil(t, res)

	cluster := &stackgresv1.SGCluster{}
	assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))
	assert.Equal(t, 3, cluster.Spec.Instances)
	assert.Equal(t, "sync", *cluster.Spec.Replication.Mode)
	assert.Equal(t, 2, *cluster.Spec.Replication.SyncInstances)
}

func Test_configureReplication_SingleInstance(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances:   1,
				Replication: vshnv1.VSHNPostgreSQLReplicationStrategy{},
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 1, cluster.Spec.Instances)
	assert.Equal(t, "async", *cluster.Spec.Replication.Mode)
}

func Test_configureReplication_SingleInstance_Sync(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: 1,
				Replication: vshnv1.VSHNPostgreSQLReplicationStrategy{
					Mode: "sync",
				},
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 1, cluster.Spec.Instances)
	assert.Equal(t, "async", *cluster.Spec.Replication.Mode)
}

func Test_configureReplication_MultiInstance_Async(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: 2,
				Replication: vshnv1.VSHNPostgreSQLReplicationStrategy{
					Mode: "async",
				},
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 2, cluster.Spec.Instances)
	assert.Equal(t, "async", *cluster.Spec.Replication.Mode)
}

func Test_configureReplication_MultiInstance_Sync(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: 2,
				Replication: vshnv1.VSHNPostgreSQLReplicationStrategy{
					Mode: "sync",
				},
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 2, cluster.Spec.Instances)
	assert.Equal(t, "sync", *cluster.Spec.Replication.Mode)
	assert.Equal(t, 1, *cluster.Spec.Replication.SyncInstances)
}

func Test_configureReplication_MultiInstance_StrictSync(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: 3,
				Replication: vshnv1.VSHNPostgreSQLReplicationStrategy{
					Mode: "strict-sync",
				},
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 3, cluster.Spec.Instances)
	assert.Equal(t, "strict-sync", *cluster.Spec.Replication.Mode)
	assert.Equal(t, 2, *cluster.Spec.Replication.SyncInstances)
}

func Test_configureReplication_MultiInstance_Default(t *testing.T) {
	ctx := context.Background()
	comp := &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: 3,
			},
		},
	}
	cluster := &stackgresv1.SGCluster{}

	cluster = configureReplication(ctx, comp, cluster)

	assert.Equal(t, 3, cluster.Spec.Instances)
	assert.Equal(t, "async", *cluster.Spec.Replication.Mode)
}
