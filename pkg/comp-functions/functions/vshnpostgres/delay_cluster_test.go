package vshnpostgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func TestDelayClusterDeployment(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/delay_cluster/01_GivenNoDependency.yaml")

	res := DelayClusterDeployment(context.TODO(), svc)
	assert.Equal(t, runtime.NewWarningResult("sgProfile is not yet ready, skipping creation of cluster"), res)

	cluster := &stackgresv1.SGCluster{}
	err := svc.GetDesiredKubeObject(cluster, "cluster")
	assert.ErrorIs(t, err, runtime.ErrNotFound)

	svc = commontest.LoadRuntimeFromFile(t, "vshn-postgres/delay_cluster/01_GivenAllDependencies.yaml")

	res = DelayClusterDeployment(context.TODO(), svc)
	assert.Nil(t, res)

	err = svc.GetDesiredKubeObject(cluster, "cluster")
	assert.Nil(t, err)
}
