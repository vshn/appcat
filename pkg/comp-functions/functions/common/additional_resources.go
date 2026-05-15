package common

import (
	"context"
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

// AddAdditionalResources deploys arbitrary Kubernetes resources into the instance namespace.
// Resources are defined in a ConfigMap in the claim namespace, referenced by ConfigMapRef.
// Each ConfigMap entry is a separate resource: key is a descriptive name, value is YAML.
// This function is a no-op when the feature flag is disabled or ConfigMapRef is not set.
func AddAdditionalResources[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *fnproto.Result {
	if !svc.GetBoolFromCompositionConfig("additionalResourcesEnabled") {
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
			return runtime.NewFatalResult(fmt.Errorf("cannot parse resource %q from configmap %q: %w", key, ar.ConfigMapRef, err))
		}
		if len(raw) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: raw}
		u.SetNamespace(instanceNamespace)

		resourceName := name + "-additional-" + stripYAMLExtension(key)
		if err := svc.SetDesiredKubeObject(u, resourceName, runtime.KubeOptionAllowDeletion); err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot set desired resource %q: %w", key, err))
		}
		log.Info("Deployed additional resource", "key", key, "kind", u.GetKind(), "name", u.GetName())
	}

	return nil
}

func stripYAMLExtension(key string) string {
	key = strings.TrimSuffix(key, ".yaml")
	key = strings.TrimSuffix(key, ".yml")
	key = strings.TrimSuffix(key, ".json")
	return key
}
