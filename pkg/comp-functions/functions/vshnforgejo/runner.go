package vshnforgejo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

const (
	runnerTokenRequestSuffix = "-runner-token-req"
	runnerReleaseSuffix      = "-runner"
	runnerChartRepository    = "oci://codeberg.org/wrenix/helm-charts"
	runnerChartName          = "forgejo-runner"
	runnerChartVersion       = "0.7.6"
)

// DeployForgejoRunner deploys a Forgejo Actions runner.
// Uses provider-http to fetch a registration token then deploys the runner Helm chart.
func DeployForgejoRunner(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	// Wait for the Forgejo release to be ready before bootstrapping the runner.
	// Two reasons: the admin API must be reachable so the token request doesn't
	// fail with connection-refused, and the admin password must have stabilised.
	// provider-http marks forProvider.headers immutable, so if the password is
	// still churning during bootstrap the token request gets wedged on an
	// immutable-field patch and only recovers via manual deletion. Once the token
	// request exists we keep reconciling it even if Forgejo briefly flaps, so we
	// don't drop it from the desired state.
	if ready, _ := svc.IsResourceReady(comp.GetName()); !ready &&
		!svc.ResourceExistsInObserved(comp.GetName()+runnerTokenRequestSuffix) {
		return runtime.NewWarningResult("waiting for forgejo to become ready before deploying runner")
	}

	credSecretName := runtime.EscapeDNS1123(comp.GetName()+"-credentials-secret", false)
	connDetails, err := svc.GetObservedComposedResourceConnectionDetails(credSecretName)
	if err != nil {
		if err == runtime.ErrNotFound {
			return runtime.NewWarningResult("waiting for admin credentials secret")
		}
		return runtime.NewWarningResult(fmt.Sprintf("cannot get admin credentials: %s", err))
	}
	password := string(connDetails["password"])
	if password == "" {
		return runtime.NewWarningResult("admin password not yet available")
	}

	forgejoURL := fmt.Sprintf("http://%s-http.%s.svc:3000", helmFullname(comp), comp.GetInstanceNamespace())

	if err := deployHTTPProviderConfig(svc, comp); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot deploy http provider config: %s", err))
	}

	if err := deployRunnerTokenRequest(svc, comp, forgejoURL, password); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot deploy runner token request: %s", err))
	}

	token, err := getObservedRunnerToken(svc, comp)
	if err != nil {
		if err == runtime.ErrNotFound {
			return runtime.NewWarningResult("waiting for runner token request to reconcile")
		}
		return runtime.NewWarningResult(fmt.Sprintf("cannot read runner token: %s", err))
	}
	if token == "" {
		return runtime.NewWarningResult("runner token response not yet available")
	}

	if err := deployRunnerRelease(svc, comp, forgejoURL, token); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot deploy runner release: %s", err))
	}

	return nil
}

func deployHTTPProviderConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) error {
	pc := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"credentials": map[string]any{
					"source": "None",
				},
			},
		},
	}
	pc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "http.crossplane.io",
		Version: "v1alpha1",
		Kind:    "ProviderConfig",
	})
	pc.SetName("http")

	return svc.SetDesiredKubeObject(pc, comp.GetName()+"-http-providerconfig")
}

func deployRunnerTokenRequest(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, forgejoURL, password string) error {
	basicAuth := base64.StdEncoding.EncodeToString([]byte("forgejo_admin:" + password))

	req := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"deletionPolicy": "Orphan",
				"providerConfigRef": map[string]any{
					"name": "http",
				},
				"forProvider": map[string]any{
					"url":    forgejoURL + "/api/v1/admin/runners/registration-token",
					"method": "GET",
					"headers": map[string]any{
						"Authorization": []any{"basic " + basicAuth},
					},
				},
			},
		},
	}
	req.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "http.crossplane.io",
		Version: "v1alpha2",
		Kind:    "DisposableRequest",
	})
	req.SetName(comp.GetName() + runnerTokenRequestSuffix)

	return svc.SetDesiredKubeObject(req, comp.GetName()+runnerTokenRequestSuffix)
}

func getObservedRunnerToken(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) (string, error) {
	observed := &unstructured.Unstructured{}
	if err := svc.GetObservedKubeObject(observed, comp.GetName()+runnerTokenRequestSuffix); err != nil {
		return "", err
	}

	responseBody, found, err := unstructured.NestedString(observed.Object, "status", "response", "body")
	if err != nil || !found || responseBody == "" {
		return "", nil
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(responseBody), &tokenResp); err != nil {
		return "", fmt.Errorf("cannot parse runner token response: %w", err)
	}

	return tokenResp.Token, nil
}

func deployRunnerRelease(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, forgejoURL, token string) error {
	values := map[string]any{
		"knownLastVersion": true,
		"runner": map[string]any{
			"config": map[string]any{
				"create":   true,
				"instance": forgejoURL,
				"name":     comp.GetName() + "-runner",
				"token":    token,
			},
		},
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return err
	}

	releaseName := comp.GetName() + runnerReleaseSuffix
	release := &xhelmv1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: releaseName,
		},
		Spec: xhelmv1.ReleaseSpec{
			RollbackRetriesLimit: ptr.To[int32](5),
			ForProvider: xhelmv1.ReleaseParameters{
				Chart: xhelmv1.ChartSpec{
					Repository: runnerChartRepository,
					Name:       runnerChartName,
					Version:    runnerChartVersion,
				},
				Namespace:           comp.GetInstanceNamespace(),
				SkipCreateNamespace: true,
				ValuesSpec: xhelmv1.ValuesSpec{
					Values: k8sruntime.RawExtension{
						Raw: vb,
					},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "helm",
				},
			},
		},
	}

	return svc.SetDesiredComposedResource(release)
}
