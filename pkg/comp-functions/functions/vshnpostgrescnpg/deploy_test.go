package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/utils/ptr"
)

const (
	testingPath = "vshn-postgres/deploy/01_default.yaml"
	plan        = "standard-1"
)

func Test_deploy(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	comp.Spec.Parameters.Backup.Enabled = ptr.To(false)
	assert.Nil(t, deployPostgresSQLUsingCNPG(ctx, comp, svc))
}

func Test_instances(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	for i := range 3 {
		i++
		comp.Spec.Parameters.Instances = i

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		assert.Equal(t, i, values["cluster"].(map[string]any)["instances"])
		// When instances > 0, hibernation should be off
		assert.Equal(t, "off", values["cluster"].(map[string]any)["annotations"].(map[string]string)["cnpg.io/hibernation"])
	}
}

func Test_hibernation(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	t.Run("instances=0 enables hibernation", func(t *testing.T) {
		comp.Spec.Parameters.Instances = 0

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		// CNPG doesn't support instances=0, so it should be set to 1
		assert.Equal(t, 1, values["cluster"].(map[string]any)["instances"])
		// Hibernation annotation should be on
		assert.Equal(t, "on", values["cluster"].(map[string]any)["annotations"].(map[string]string)["cnpg.io/hibernation"])
	})

	t.Run("instances>0 disables hibernation", func(t *testing.T) {
		comp.Spec.Parameters.Instances = 2

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		// Instances should match spec
		assert.Equal(t, 2, values["cluster"].(map[string]any)["instances"])
		// Hibernation annotation should be off
		assert.Equal(t, "off", values["cluster"].(map[string]any)["annotations"].(map[string]string)["cnpg.io/hibernation"])
	})
}

func Test_version(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	for _, v := range []string{
		"15", "16", "17",
	} {
		comp.Spec.Parameters.Service.MajorVersion = v
		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		assert.Equal(t, v, values["version"].(map[string]string)["postgresql"])
	}
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
