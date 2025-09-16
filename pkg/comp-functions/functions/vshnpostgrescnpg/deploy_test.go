package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const (
	testingPath = "vshn-postgres/deploy/01_default.yaml"
	plan        = "standard-1"
)

// Note: We are only testing stuff regarding the CNPG helm deployment.
// Other things, such as certificates etc., are already handled in other tests
func Test_deploy(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	assert.Nil(t, deployPostgresSQLUsingCNPG(ctx, comp, svc))
}

func Test_sizing(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	comp.Spec.Parameters.Size.Plan = plan

	values, err := createCnpgHelmValues(ctx, svc, comp)
	assert.NoError(t, err)
	assert.NotNil(t, values)

	t.Log("Checking if resources correspond to plan")
	res, err := getResourcesForPlan(ctx, svc, comp, plan)
	assert.NoError(t, err)
	assert.Equal(t, "16Gi", res.Disk.String())
	assert.Equal(t, "250m", res.CPU.String())
	assert.Equal(t, "250m", res.ReqCPU.String())
	assert.Equal(t, "1Gi", res.Mem.String())
	assert.Equal(t, "1Gi", res.ReqMem.String())

	t.Log("... as well as in the helm values")
	assert.Equal(t, res.Disk.String(), values["cluster"].(map[string]any)["storage"].(map[string]any)["size"])
	resvalues := values["cluster"].(map[string]any)["resources"].(map[string]any)
	assert.Equal(t, res.CPU.String(), resvalues["limits"].(map[string]any)["cpu"])
	assert.Equal(t, res.ReqCPU.String(), resvalues["requests"].(map[string]any)["cpu"])
	assert.Equal(t, res.Mem.String(), resvalues["limits"].(map[string]any)["memory"])
	assert.Equal(t, res.ReqMem.String(), resvalues["requests"].(map[string]any)["memory"])

	t.Log("Setting our own disk size despite a plan being set")
	const ourDiskSize = "32Gi"
	comp.Spec.Parameters.Size.Disk = ourDiskSize
	res, err = getResourcesForPlan(ctx, svc, comp, plan)
	assert.NoError(t, err)

	values, err = createCnpgHelmValues(ctx, svc, comp)
	assert.NoError(t, err)
	assert.NotNil(t, values)
	assert.Equal(t, ourDiskSize, res.Disk.String())
	assert.Equal(t, ourDiskSize, values["cluster"].(map[string]any)["storage"].(map[string]any)["size"])
}

// Obtain svc and comp for CNPG tests
func getSvcCompCnpg(testing *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNPostgreSQL) {
	svc, comp := getPostgreSqlComp(testing, testingPath)
	return svc, comp
}

func getPostgreSqlComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNPostgreSQL) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
