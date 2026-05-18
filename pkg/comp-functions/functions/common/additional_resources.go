package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	fnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

// AllowedResourceRule mirrors the shape of an RBAC PolicyRule for the subset
// of fields we use: API groups and resource kinds. A resource is allowed if it
// matches at least one rule on both axes. The wildcard "*" matches anything.
type AllowedResourceRule struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
}

// AddAdditionalResources deploys arbitrary Kubernetes resources into the instance namespace.
// Resources are defined in a ConfigMap in the claim namespace, referenced by ConfigMapRef.
// Each ConfigMap entry is a separate resource: key is a descriptive name, value is YAML.
//
// The composition config key "additionalResourcesAllowed" is a JSON-encoded list of
// AllowedResourceRule entries (RBAC-style apiGroups + kinds). A resource is deployed
// only if it matches at least one rule on both axes; everything else is rejected with
// a warning. An empty or unset allowlist disables the feature entirely.
func AddAdditionalResources[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *fnproto.Result {
	allowed, err := parseAllowedResources(svc.Config.Data["additionalResourcesAllowed"])
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot parse additionalResourcesAllowed: %w", err))
	}
	if len(allowed) == 0 {
		return nil
	}

	log := controllerruntime.LoggerFrom(ctx)

	if err := svc.GetObservedComposite(obj); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	getter, ok := any(obj).(AdditionalResourcesGetter)
	if !ok {
		return runtime.NewWarningResult(fmt.Sprintf("type %T doesn't implement AdditionalResourcesGetter", obj))
	}

	ar := getter.GetAdditionalResources()
	if ar.ConfigMapRef == "" {
		return nil
	}

	claimNamespace := getter.GetClaimNamespace()
	instanceNamespace := getter.GetInstanceNamespace()
	name := obj.GetName()
	observerName := name + "-additional-resources-cm"

	// Set up an observe-only xkube Object for the ConfigMap in the claim namespace.
	// On the first reconcile this won't exist in observed state yet; we return nil and
	// wait for the next pass.
	observerCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ar.ConfigMapRef,
			Namespace: claimNamespace,
		},
	}
	if err := svc.SetDesiredKubeObject(observerCM, observerName,
		runtime.KubeOptionAddLabels(map[string]string{runtime.ProviderConfigIgnoreLabel: "true"}),
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
	); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set observer for configmap %q: %w", ar.ConfigMapRef, err))
	}

	observedCM := &corev1.ConfigMap{}
	if err := svc.GetObservedKubeObject(observedCM, observerName); err != nil {
		if err == runtime.ErrNotFound {
			log.Info("ConfigMap not yet observed, will retry on next reconcile", "configMapRef", ar.ConfigMapRef)
			return nil
		}
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed configmap %q: %w", ar.ConfigMapRef, err))
	}

	for key, yamlContent := range observedCM.Data {
		raw := map[string]interface{}{}
		if err := sigsyaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot parse resource %q from configmap %q: %v", key, ar.ConfigMapRef, err)))
			continue
		}
		if len(raw) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: raw}

		if err := validateAdditionalResource(u, allowed); err != nil {
			log.Info("Rejected additional resource", "key", key, "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "reason", err.Error())
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("resource %q in configmap %q is not allowed: %v", key, ar.ConfigMapRef, err)))
			continue
		}

		u.SetNamespace(instanceNamespace)

		resourceName := name + "-additional-" + stripYAMLExtension(key)
		if err := svc.SetDesiredKubeObject(u, resourceName, runtime.KubeOptionAllowDeletion); err != nil {
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot set desired resource %q: %v", key, err)))
			continue
		}
		log.Info("Deployed additional resource", "key", key, "kind", u.GetKind(), "name", u.GetName())
	}

	return nil
}

// parseAllowedResources decodes the JSON-encoded allowlist from the composition
// config. An empty or whitespace-only value yields a nil slice (feature disabled).
func parseAllowedResources(raw string) ([]AllowedResourceRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var rules []AllowedResourceRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// validateAdditionalResource returns an error if the resource doesn't match any rule.
func validateAdditionalResource(u *unstructured.Unstructured, rules []AllowedResourceRule) error {
	apiVersion := u.GetAPIVersion()
	// apiVersion is either "group/version" or just "version" for core resources.
	group := ""
	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		group = apiVersion[:idx]
	}
	kind := u.GetKind()
	for _, r := range rules {
		if matchesAny(r.APIGroups, group) && matchesAny(r.Kinds, kind) {
			return nil
		}
	}
	return fmt.Errorf("resource %s/%s is not in allowlist", group, kind)
}

// matchesAny returns true if value is present in list, or if list contains "*".
func matchesAny(list []string, value string) bool {
	for _, v := range list {
		if v == "*" || v == value {
			return true
		}
	}
	return false
}

func stripYAMLExtension(key string) string {
	key = strings.TrimSuffix(key, ".yaml")
	key = strings.TrimSuffix(key, ".yml")
	key = strings.TrimSuffix(key, ".json")
	return key
}
