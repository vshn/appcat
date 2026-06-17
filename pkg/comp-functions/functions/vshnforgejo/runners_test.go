package vshnforgejo

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// setObservedForgejoComposite injects comp as the observed composite so that
// steps calling svc.GetObservedComposite see the test's modifications.
func setObservedForgejoComposite(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) error {
	v := reflect.ValueOf(svc).Elem()
	val := v.FieldByName("observedComposite")
	val = reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem()

	ccomp := composite.New()
	jcomp, err := json.Marshal(comp)
	if err != nil {
		return err
	}
	if err := ccomp.Unstructured.UnmarshalJSON(jcomp); err != nil {
		return err
	}
	val.Set(reflect.ValueOf(ccomp))
	return nil
}

func testRunner() vshnv1.VSHNForgejoRunnerSpec {
	return vshnv1.VSHNForgejoRunnerSpec{
		Name:     "default",
		Replicas: 2,
		Labels:   []string{"ubuntu-latest", "linux"},
	}
}

func TestNewRunnerRequest_RegisterAndDeregister(t *testing.T) {
	_, comp, _ := bootstrapTest(t)
	runner := testRunner()
	base := comp.GetName() + "-default-runner"

	req := newRunnerRequest(comp, runner)

	assert.Equal(t, base+"-reg", req.GetName())
	assert.Equal(t, base+"-http", req.Spec.ProviderConfigReference.Name)

	var create, observe, remove bool
	for _, m := range req.Spec.ForProvider.Mappings {
		switch m.Action {
		case "CREATE":
			create = true
			assert.Equal(t, "POST", m.Method)
			assert.True(t, strings.Contains(m.URL, "/api/v1/admin/actions/runners"), "create URL must hit runners endpoint, got %q", m.URL)
			assert.True(t, strings.Contains(m.Body, `"labels":["ubuntu-latest","linux"]`), "create body must carry the runner labels, got %q", m.Body)
		case "OBSERVE":
			observe = true
			assert.Equal(t, "GET", m.Method)
			assert.True(t, strings.Contains(m.URL, "/api/v1/admin/actions/runners/"), "observe URL must target a runner id, got %q", m.URL)
		case "REMOVE":
			remove = true
			assert.Equal(t, "DELETE", m.Method)
			assert.True(t, strings.Contains(m.URL, "/api/v1/admin/actions/runners/"), "remove URL must target a runner id, got %q", m.URL)
		}
	}
	assert.True(t, create, "must have a CREATE mapping (register)")
	assert.True(t, observe, "must have an OBSERVE mapping (required by provider-http)")
	assert.True(t, remove, "must have a REMOVE mapping (deregister)")

	// The create-response body is injected into the .runner secret consumed by the chart.
	assert.Len(t, req.Spec.ForProvider.SecretInjectionConfigs, 1)
	inj := req.Spec.ForProvider.SecretInjectionConfigs[0]
	assert.Equal(t, base+"-secret", inj.SecretRef.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), inj.SecretRef.Namespace)
	assert.True(t, inj.SetOwnerReference, "secret must be owned by the Request for GC")
	assert.Len(t, inj.KeyMappings, 1)
	assert.Equal(t, ".runner", inj.KeyMappings[0].SecretKey)
}

func TestBasicAuthHeader(t *testing.T) {
	// "forgejo_admin:secret" base64-encoded
	assert.Equal(t, "Basic Zm9yZ2Vqb19hZG1pbjpzZWNyZXQ=", basicAuthHeader("forgejo_admin", "secret"))
}

func TestNewRunnerProviderConfig_ReferencesAuthSecret(t *testing.T) {
	_, comp, _ := bootstrapTest(t)
	runner := testRunner()
	base := comp.GetName() + "-default-runner"

	pc := newRunnerProviderConfig(comp, runner)

	assert.Equal(t, base+"-http", pc.GetName())

	creds := pc.Spec.Credentials
	assert.Equal(t, xpv1.CredentialsSourceSecret, creds.Source)
	assert.NotNil(t, creds.SecretRef)
	assert.Equal(t, base+"-auth", creds.SecretRef.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), creds.SecretRef.Namespace)
	assert.Equal(t, "Authorization", creds.SecretRef.Key)
}

func TestNewRunnerRelease_RootlessAndResources(t *testing.T) {
	svc, comp, _ := bootstrapTest(t)
	svc.Config.Data["runnerPlans"] = `{"runner-mini":{"size":{"cpu":"500m","memory":"1Gi","disk":"0"}}}`
	svc.Config.Data["forgejoRunnerDefaultPlan"] = "runner-mini"
	svc.Config.Data["runnerChartName"] = "forgejo-runner"
	svc.Config.Data["runnerChartRepository"] = "https://code.forgejo.org/forgejo-helm"
	svc.Config.Data["runnerChartVersion"] = "1.0.0"
	runner := testRunner()
	base := comp.GetName() + "-default-runner"

	rel, err := newRunnerRelease(context.TODO(), svc, comp, runner)
	assert.NoError(t, err)
	assert.Equal(t, base, rel.GetName())
	assert.Equal(t, "forgejo-runner", rel.Spec.ForProvider.Chart.Name)
	assert.Equal(t, comp.GetInstanceNamespace(), rel.Spec.ForProvider.Namespace)

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(rel.Spec.ForProvider.Values.Raw, &values))

	// rootless: runs as non-root, no privilege escalation
	sc := values["statefulset"].(map[string]any)["securityContext"].(map[string]any)
	assert.Equal(t, true, sc["runAsNonRoot"])
	assert.Equal(t, false, sc["allowPrivilegeEscalation"])

	// deployed resources drop the composite prefix
	assert.Equal(t, "default-runner", values["fullnameOverride"])

	// existingSecret wired to the .runner secret
	cfg := values["runner"].(map[string]any)["config"].(map[string]any)
	assert.Equal(t, base+"-secret", cfg["existingSecret"])

	// replicaCount + plan resources
	assert.Equal(t, float64(2), values["replicaCount"])
	lim := values["resources"].(map[string]any)["limits"].(map[string]any)
	assert.Equal(t, "500m", lim["cpu"])
	assert.Equal(t, "1Gi", lim["memory"])
}

func TestDeployRunners_NoRunnersComposesNothing(t *testing.T) {
	svc, comp, _ := bootstrapTest(t)
	comp.Spec.Parameters.Runners = nil
	assert.NoError(t, setObservedForgejoComposite(svc, comp))

	res := DeployRunners(context.TODO(), comp, svc)
	assert.Nil(t, res)

	rel := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(rel, comp.GetName()+"-default-runner")
	assert.Error(t, err, "no runner release should be composed when there are no runners")
}

func TestDeployRunners_NotReadyRequeues(t *testing.T) {
	svc, comp, _ := bootstrapTest(t)
	comp.Spec.Parameters.Runners = []vshnv1.VSHNForgejoRunnerSpec{testRunner()}
	assert.NoError(t, setObservedForgejoComposite(svc, comp))
	// No observed forgejo release is present, so the instance is not ready.

	res := DeployRunners(context.TODO(), comp, svc)
	assert.NotNil(t, res)
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, res.Severity)

	rel := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(rel, comp.GetName()+"-default-runner")
	assert.Error(t, err, "no runner release should be composed before the instance is ready")
}
