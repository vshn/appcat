package vshnforgejo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	xhttpcommon "github.com/vshn/appcat/v4/apis/http/common"
	xhttp "github.com/vshn/appcat/v4/apis/http/request/v1alpha2"
	xhttppc "github.com/vshn/appcat/v4/apis/http/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// runnerAuthHeaderKey is the secret key holding the full Authorization header value
// consumed by the per-runner provider-http ProviderConfig.
const runnerAuthHeaderKey = "Authorization"

// forgejoAdminUsername is the static admin username configured for the Forgejo
// instance (see DeployForgejo). It is used for HTTP Basic auth against the admin API.
const forgejoAdminUsername = "forgejo_admin"

// Resource name suffixes, all relative to a runner's base name (see runnerBaseName).
const (
	runnerReqSuffix      = "-reg"
	runnerSecretSuffix   = "-secret"
	runnerPCSuffix       = "-http"
	runnerAuthSuffix     = "-auth"
	runnerRegistrationEP = "/api/v1/admin/actions/runners"
)

// runnerBaseName returns the base name shared by all resources of a single runner
// group. The runner's name keeps the resources of different runner groups distinct.
func runnerBaseName(comp *vshnv1.VSHNForgejo, runner vshnv1.VSHNForgejoRunnerSpec) string {
	return comp.GetName() + "-" + runner.Name + "-runner"
}

// DeployRunners registers and deploys every configured Forgejo runner group, and
// cleans up the resources of removed groups (composing nothing for them, so
// Crossplane garbage-collects the resources and the provider-http Request REMOVE
// mapping deregisters the runner).
func DeployRunners(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	// No runners configured: compose nothing. Previously-composed runner resources
	// are garbage-collected; each Request's REMOVE mapping deregisters its runner.
	if len(comp.Spec.Parameters.Runners) == 0 {
		svc.Log.Info("No runners configured, skipping (resources will be garbage-collected)")
		return nil
	}

	// Wait for the Forgejo instance release to be ready before registering.
	if !forgejoReleaseReady(svc, comp) {
		return runtime.NewWarningResult("forgejo instance not ready yet, requeueing runner registration")
	}

	authHeader, ok := runnerAdminAuthHeader(svc, comp)
	if !ok {
		return runtime.NewWarningResult("forgejo admin credentials not available yet, requeueing runner registration")
	}

	for _, runner := range comp.Spec.Parameters.Runners {
		base := runnerBaseName(comp, runner)

		// The registration Request needs the auth secret and ProviderConfig during
		// its own deletion (the REMOVE mapping deregisters the runner). Protect them
		// by the Request so Crossplane deletes the Request first and only then the
		// credentials.
		if err := svc.SetDesiredKubeObject(newRunnerAuthSecret(comp, runner, authHeader), base+runnerAuthSuffix,
			runtime.KubeOptionAllowDeletion, runtime.KubeOptionProtectedBy(base+runnerReqSuffix)); err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot add auth secret for runner %q: %s", runner.Name, err))
		}

		if err := svc.SetDesiredKubeObject(newRunnerProviderConfig(comp, runner), base+runnerPCSuffix,
			runtime.KubeOptionAllowDeletion, runtime.KubeOptionProtectedBy(base+runnerReqSuffix)); err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot add provider config for runner %q: %s", runner.Name, err))
		}

		if err := svc.SetDesiredComposedResource(newRunnerRequest(comp, runner)); err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot add registration request for runner %q: %s", runner.Name, err))
		}

		rel, err := newRunnerRelease(ctx, svc, comp, runner)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot build release for runner %q: %s", runner.Name, err))
		}
		if err := svc.SetDesiredComposedResource(rel); err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot add release for runner %q: %s", runner.Name, err))
		}
	}

	svc.Log.Info("Runner resources composed", "count", len(comp.Spec.Parameters.Runners))
	return nil
}

// forgejoReleaseReady reports whether the main Forgejo Helm release is observed Ready.
func forgejoReleaseReady(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) bool {
	rel := &xhelmv1.Release{}
	if err := svc.GetObservedComposedResource(rel, comp.GetName()); err != nil {
		return false
	}
	return rel.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue
}

// newRunnerAuthSecret builds the secret holding the Authorization header value used
// by the runner's provider-http ProviderConfig to authenticate against the admin API.
func newRunnerAuthSecret(comp *vshnv1.VSHNForgejo, runner vshnv1.VSHNForgejoRunnerSpec, authHeader string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runnerBaseName(comp, runner) + runnerAuthSuffix,
			Namespace: comp.GetInstanceNamespace(),
		},
		StringData: map[string]string{
			runnerAuthHeaderKey: authHeader,
		},
	}
}

// basicAuthHeader returns an HTTP Basic Authorization header value for the given
// credentials.
func basicAuthHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

// runnerAdminAuthHeader builds the Basic Authorization header for the Forgejo admin
// API from the observed admin credentials secret. It returns ok=false if the
// password is not yet available (e.g. the credentials secret has not been applied to
// the instance namespace yet).
func runnerAdminAuthHeader(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) (string, bool) {
	secret := &corev1.Secret{}
	if err := svc.GetObservedKubeObject(secret, comp.GetName()+"-credentials-secret"); err != nil {
		return "", false
	}
	password := string(secret.Data["password"])
	if password == "" {
		return "", false
	}
	return basicAuthHeader(forgejoAdminUsername, password), true
}

// forgejoAPIBaseURL returns the in-cluster base URL of the Forgejo HTTP service.
func forgejoAPIBaseURL(comp *vshnv1.VSHNForgejo) string {
	return fmt.Sprintf("http://%s-http.%s.svc:3000", helmFullname(comp), comp.GetInstanceNamespace())
}

// newRunnerProviderConfig builds a per-runner provider-http ProviderConfig that
// authenticates admin API calls. provider-http reads the credential from the
// referenced secret and injects it as the Authorization header on each request.
//
// It is applied to the cluster through provider-kubernetes (objects.kubernetes.crossplane.io)
// via svc.SetDesiredKubeObject in DeployRunners.
func newRunnerProviderConfig(comp *vshnv1.VSHNForgejo, runner vshnv1.VSHNForgejoRunnerSpec) *xhttppc.ProviderConfig {
	base := runnerBaseName(comp, runner)
	return &xhttppc.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: base + runnerPCSuffix,
		},
		Spec: xhttppc.ProviderConfigSpec{
			Credentials: xhttppc.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      base + runnerAuthSuffix,
							Namespace: comp.GetInstanceNamespace(),
						},
						Key: runnerAuthHeaderKey,
					},
				},
			},
		},
	}
}

// newRunnerRelease builds the forgejo-runner Helm release for a single runner group.
// It uses dedicated runnerChart* config keys and the runner plans (separate from the
// Forgejo instance plans). Runner pods run rootless so they are compatible with the
// restricted Pod Security Standard / OpenShift restricted SCC.
func newRunnerRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, runner vshnv1.VSHNForgejoRunnerSpec) (*xhelmv1.Release, error) {
	base := runnerBaseName(comp, runner)

	plan := runner.GetPlan(svc.Config.Data["forgejoRunnerDefaultPlan"])
	planResources, err := utils.FetchPlansFromConfigByKey(ctx, svc, "runnerPlans", plan)
	if err != nil {
		return nil, fmt.Errorf("could not fetch runner plans: %w", err)
	}

	res, errs := common.GetResources(&runner.Size, planResources)
	if len(errs) > 0 {
		svc.Log.Error(fmt.Errorf("could not get runner resources"), "errors", errs)
	}

	values := map[string]any{
		// The Crossplane Release MR name is composite-prefixed (cluster-unique), but
		// the deployed resources only need to be unique within the instance namespace.
		// Override the chart fullname so the workload names drop the composite prefix.
		"fullnameOverride": runner.Name + "-runner",
		"replicaCount":     runner.Replicas,
		"runner": map[string]any{
			"config": map[string]any{
				// existingSecret provides /etc/runner/.runner (the registration:
				// id, uuid, name, token, address, labels) populated by provider-http.
				"existingSecret": base + runnerSecretSuffix,
			},
		},
		// rootless: compatible with the restricted Pod Security Standard / OpenShift restricted SCC
		"statefulset": map[string]any{
			"securityContext": map[string]any{
				"runAsNonRoot":             true,
				"allowPrivilegeEscalation": false,
				"seccompProfile":           map[string]any{"type": "RuntimeDefault"},
				"capabilities":             map[string]any{"drop": []any{"ALL"}},
			},
		},
		"resources": map[string]any{
			"limits": map[string]any{
				"cpu":    res.CPU.String(),
				"memory": res.Mem.String(),
			},
			"requests": map[string]any{
				"cpu":    res.ReqCPU.String(),
				"memory": res.ReqMem.String(),
			},
		},
		"knownLastVersion": true,
	}

	if reg := svc.Config.Data["imageRegistry"]; reg != "" {
		values["image"] = map[string]any{"registry": reg}
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	release := &xhelmv1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: base,
			Labels: map[string]string{
				runtime.WebhookAllowDeletionLabel: "true",
			},
		},
		Spec: xhelmv1.ReleaseSpec{
			RollbackRetriesLimit: ptr.To[int32](10),
			ForProvider: xhelmv1.ReleaseParameters{
				Chart: xhelmv1.ChartSpec{
					Repository: svc.Config.Data["runnerChartRepository"],
					Version:    svc.Config.Data["runnerChartVersion"],
					Name:       svc.Config.Data["runnerChartName"],
				},
				Namespace: comp.GetInstanceNamespace(),
				ValuesSpec: xhelmv1.ValuesSpec{
					Values: k8sruntime.RawExtension{Raw: vb},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "helm"},
			},
		},
	}

	return release, nil
}

// newRunnerRequest builds the provider-http Request that registers a runner group via
// the Forgejo admin API on CREATE and deregisters it on REMOVE (when the resource is
// garbage-collected). The registration token from the create response is injected
// into the .runner secret consumed by the runner Helm chart.
//
// Authentication is performed via the runner's provider-http ProviderConfig
// (see newRunnerProviderConfig); the Request only references it by name.
func newRunnerRequest(comp *vshnv1.VSHNForgejo, runner vshnv1.VSHNForgejoRunnerSpec) *xhttp.Request {
	base := runnerBaseName(comp, runner)
	apiBase := forgejoAPIBaseURL(comp)
	runnerName := base

	// provider-http evaluates the URL and body fields as jq expressions. URLs must
	// therefore be jq string literals (wrapped in quotes) so they are returned
	// verbatim, and the runner id is injected with jq string interpolation \(...)
	// from the stored create response.
	createURL := fmt.Sprintf("%q", apiBase+runnerRegistrationEP)
	runnerByIDURL := fmt.Sprintf(`"%s%s/\(.response.body.id)"`, apiBase, runnerRegistrationEP)

	// labels are marshalled to a JSON array and embedded verbatim in the jq object
	// construction and the registration POST body (both are valid jq/JSON).
	labelsJSON, err := json.Marshal(runner.Labels)
	if err != nil {
		labelsJSON = []byte("[]")
	}

	// The .runner file written into the secret is the act_runner registration:
	// the create response (id, uuid, token) merged with the runner name, the Forgejo
	// instance address it connects to and the runner labels.
	// Only write it when the token is present (the OBSERVE response has none),
	// otherwise preserve the existing value.
	runnerFileJQ := fmt.Sprintf(`if .body.token then (.body + {name: %q, address: %q, labels: %s} | tojson) else empty end`, runnerName, apiBase, labelsJSON)

	return &xhttp.Request{
		ObjectMeta: metav1.ObjectMeta{
			Name: base + runnerReqSuffix,
		},
		Spec: xhttp.RequestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: base + runnerPCSuffix,
				},
			},
			ForProvider: xhttp.RequestParameters{
				Payload: xhttp.Payload{
					BaseUrl: apiBase,
				},
				// Header values are evaluated as jq expressions, so the literal
				// content type must be a quoted jq string. Forgejo rejects the POST
				// body with HTTP 422 without an explicit Content-Type.
				Headers: map[string][]string{
					"Content-Type": {`"application/json"`},
				},
				Mappings: []xhttp.Mapping{
					{
						Action: xhttp.ActionCreate,
						Method: "POST",
						URL:    createURL,
						Body:   fmt.Sprintf(`{"name":%q,"labels":%s}`, runnerName, labelsJSON),
					},
					{
						// provider-http requires an OBSERVE (GET) mapping to determine whether
						// the runner already exists. The id is resolved from the stored create response.
						Action: xhttp.ActionObserve,
						Method: "GET",
						URL:    runnerByIDURL,
					},
					{
						Action: xhttp.ActionRemove,
						Method: "DELETE",
						// The runner id is resolved by provider-http from the stored create response.
						URL: runnerByIDURL,
					},
				},
				SecretInjectionConfigs: []xhttpcommon.SecretInjectionConfig{
					{
						SecretRef: xhttpcommon.SecretRef{
							Name:      base + runnerSecretSuffix,
							Namespace: comp.GetInstanceNamespace(),
						},
						// Own the secret by this Request so Kubernetes garbage-collects it
						// when the runner is decommissioned (the Request is removed).
						SetOwnerReference: true,
						KeyMappings: []xhttpcommon.KeyInjection{
							{
								// The runner chart mounts this secret at /etc/runner and copies
								// /etc/runner/.runner, so the key must be ".runner". It holds the
								// act_runner registration file (create response id, uuid, token plus
								// the runner name, instance address and labels).
								SecretKey:            ".runner",
								ResponseJQ:           runnerFileJQ,
								MissingFieldStrategy: xhttpcommon.PreserveMissingField,
							},
						},
					},
				},
			},
		},
	}
}
