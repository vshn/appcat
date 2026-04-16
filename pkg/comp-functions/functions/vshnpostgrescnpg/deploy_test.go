package vshnpostgrescnpg

import (
	"context"
	"testing"

	"encoding/json"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	testingPath         = "vshn-postgres/deploy/01_default.yaml"
	lbCertWithIPPath    = "vshn-postgres/deploy/06_lb_cert_with_ip.yaml"
	lbCertWithoutIPPath = "vshn-postgres/deploy/07_lb_cert_without_ip.yaml"
	plan                = "standard-1"
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

func Test_enablePDB(t *testing.T) {
	svc, comp := getSvcCompCnpg(t)
	ctx := context.TODO()

	t.Run("PDB disabled for single instance", func(t *testing.T) {
		comp.Spec.Parameters.Instances = 1

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.Equal(t, false, values["cluster"].(map[string]any)["enablePDB"])
	})

	t.Run("PDB enabled for multiple instances", func(t *testing.T) {
		comp.Spec.Parameters.Instances = 2

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.Equal(t, true, values["cluster"].(map[string]any)["enablePDB"])
	})

	t.Run("PDB disabled for paused instance", func(t *testing.T) {
		comp.Spec.Parameters.Instances = 0

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.Equal(t, false, values["cluster"].(map[string]any)["enablePDB"])
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

func Test_pinImageTag(t *testing.T) {
	ctx := context.TODO()

	t.Run("no pinImageTag uses major version tag", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Service.MajorVersion = "15"
		comp.Spec.Parameters.Maintenance.PinImageTag = ""

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		imageCatalog := values["imageCatalog"].(map[string]any)
		images := imageCatalog["images"].([]map[string]string)

		assert.Len(t, images, 1)
		assert.Equal(t, "15", images[0]["major"])
		assert.Equal(t, "ghcr.io/cloudnative-pg/postgresql:15", images[0]["image"])
		assert.Equal(t, "15", comp.Status.CurrentVersion)
	})

	t.Run("pinImageTag overrides major version tag in ImageCatalog", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Service.MajorVersion = "15"
		comp.Spec.Parameters.Maintenance.PinImageTag = "15.13"

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		imageCatalog := values["imageCatalog"].(map[string]any)
		images := imageCatalog["images"].([]map[string]string)

		assert.Len(t, images, 1)
		assert.Equal(t, "15", images[0]["major"])
		assert.Equal(t, "ghcr.io/cloudnative-pg/postgresql:15.13", images[0]["image"])
		assert.Equal(t, "15.13", comp.Status.CurrentVersion)
	})

	t.Run("pinImageTag with different major version", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Service.MajorVersion = "16"
		comp.Spec.Parameters.Maintenance.PinImageTag = "16.4"

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		imageCatalog := values["imageCatalog"].(map[string]any)
		images := imageCatalog["images"].([]map[string]string)

		assert.Len(t, images, 1)
		assert.Equal(t, "16", images[0]["major"])
		assert.Equal(t, "ghcr.io/cloudnative-pg/postgresql:16.4", images[0]["image"])
		assert.Equal(t, "16.4", comp.Status.CurrentVersion)
	})

	t.Run("majorVersion is correctly set in helm values", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Service.MajorVersion = "17"
		comp.Spec.Parameters.Maintenance.PinImageTag = "17.2"

		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)

		version := values["version"].(map[string]string)
		assert.Equal(t, "17", version["postgresql"])

		cluster := values["cluster"].(map[string]any)
		imageCatalogRef := cluster["imageCatalogRef"].(map[string]string)
		assert.Equal(t, "ImageCatalog", imageCatalogRef["kind"])
		assert.Equal(t, "postgresql", imageCatalogRef["name"])

		assert.Equal(t, "17.2", comp.Status.CurrentVersion)
	})
}

// toUnstructured round-trips values through JSON so that all intermediate maps
// become map[string]interface{}, which lets us use the unstructured nested helpers.
func toUnstructured(t *testing.T, values map[string]any) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(values)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func Test_addLoadbalancerConfig(t *testing.T) {
	t.Run("no-op when serviceType is ClusterIP", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Network.ServiceType = "ClusterIP"

		values := map[string]any{
			"cluster": map[string]any{},
		}

		require.NoError(t, addLoadbalancerConfig(svc, comp, values))

		u := toUnstructured(t, values)
		_, found, err := unstructured.NestedMap(u, "cluster", "services")
		require.NoError(t, err)
		assert.False(t, found, "services should not be set for ClusterIP")
	})

	t.Run("sets services when serviceType is LoadBalancer", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Network.ServiceType = string(corev1.ServiceTypeLoadBalancer)

		values := map[string]any{
			"cluster": map[string]any{},
		}

		require.NoError(t, addLoadbalancerConfig(svc, comp, values))

		u := toUnstructured(t, values)
		additional, found, err := unstructured.NestedSlice(u, "cluster", "services", "additional")
		require.NoError(t, err)
		require.True(t, found, "services.additional should be set for LoadBalancer")
		require.Len(t, additional, 1)

		entry := additional[0].(map[string]interface{})
		assert.Equal(t, "rw", entry["selectorType"])

		name, found, err := unstructured.NestedString(entry, "serviceTemplate", "metadata", "name")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "primary", name)

		svcType, found, err := unstructured.NestedString(entry, "serviceTemplate", "spec", "type")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, string(corev1.ServiceTypeLoadBalancer), svcType)
	})

	t.Run("annotations are empty when no config is set", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Network.ServiceType = string(corev1.ServiceTypeLoadBalancer)

		values := map[string]any{
			"cluster": map[string]any{},
		}

		require.NoError(t, addLoadbalancerConfig(svc, comp, values))

		u := toUnstructured(t, values)
		additional, _, err := unstructured.NestedSlice(u, "cluster", "services", "additional")
		require.NoError(t, err)
		entry := additional[0].(map[string]interface{})

		annotations, _, err := unstructured.NestedStringMap(entry, "serviceTemplate", "metadata", "annotations")
		require.NoError(t, err)
		assert.Empty(t, annotations)
	})

	t.Run("includes loadbalancer annotations from config", func(t *testing.T) {
		svc, comp := getSvcCompCnpg(t)
		comp.Spec.Parameters.Network.ServiceType = string(corev1.ServiceTypeLoadBalancer)

		svc.Config.Data["loadbalancerAnnotations"] = "service.beta.kubernetes.io/load-balancer-type: nlb"

		values := map[string]any{
			"cluster": map[string]any{},
		}

		require.NoError(t, addLoadbalancerConfig(svc, comp, values))

		u := toUnstructured(t, values)
		additional, _, err := unstructured.NestedSlice(u, "cluster", "services", "additional")
		require.NoError(t, err)
		entry := additional[0].(map[string]interface{})

		annotations, found, err := unstructured.NestedStringMap(entry, "serviceTemplate", "metadata", "annotations")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "nlb", annotations["service.beta.kubernetes.io/load-balancer-type"])
	})
}

func Test_createCerts_LoadBalancer(t *testing.T) {
	t.Run("cert marked ready when LB IP is in certificate IPAddresses", func(t *testing.T) {
		svc, comp := getPostgreSqlComp(t, lbCertWithIPPath)

		err := createCerts(comp, svc)
		require.NoError(t, err)

		desired := svc.GetAllDesired()
		certRes, ok := desired[resource.Name("certificate")]
		require.True(t, ok, "certificate resource should exist in desired state")
		assert.Equal(t, resource.ReadyTrue, certRes.Ready, "certificate should be marked ready when LB IP is in cert DNSNames")
	})

	t.Run("cert marked unready when LB IP is not in certificate IPAddresses", func(t *testing.T) {
		svc, comp := getPostgreSqlComp(t, lbCertWithoutIPPath)

		err := createCerts(comp, svc)
		require.NoError(t, err)

		desired := svc.GetAllDesired()
		certRes, ok := desired[resource.Name("certificate")]
		require.True(t, ok, "certificate resource should exist in desired state")
		assert.Equal(t, resource.ReadyFalse, certRes.Ready, "certificate should be marked unready when LB IP is not in cert DNSNames")
	})

	t.Run("LB IP is added to certificate IPAddresses", func(t *testing.T) {
		svc, comp := getPostgreSqlComp(t, lbCertWithIPPath)

		err := createCerts(comp, svc)
		require.NoError(t, err)

		cert := &cmv1.Certificate{}
		require.NoError(t, svc.GetDesiredKubeObject(cert, "certificate"))

		assert.Contains(t, cert.Spec.IPAddresses, "10.0.0.1", "LB IP should be in certificate IPAddresses")
		assert.NotContains(t, cert.Spec.DNSNames, "10.0.0.1", "LB IP should not be in DNSNames")
		assert.Contains(t, cert.Spec.DNSNames, "postgresql-rw."+comp.GetInstanceNamespace()+".svc.cluster.local")
		assert.Contains(t, cert.Spec.DNSNames, "postgresql-rw."+comp.GetInstanceNamespace()+".svc")
	})
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
