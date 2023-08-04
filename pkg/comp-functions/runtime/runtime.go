package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var s = pkg.SetupScheme()

type contextKey int

// Runtime a struct which encapsulates crossplane FunctionIO
type Runtime struct {
	io       xfnv1alpha1.FunctionIO
	Observed ObservedResources
	Desired  DesiredResources
	Config   *corev1.ConfigMap
}

type Resource interface {
	GetName() string
	GetRaw() []byte
	SetRaw([]byte)
	// Returns this resource as a DesiredResource.
	// Depending on the concrete underlying type, not all fields are populated.
	GetDesiredResource() xfnv1alpha1.DesiredResource
	// Returns this resource as an ObservedResource.
	// Depending on the concrete underlying type, not all fields are populated.
	GetObservedResource() xfnv1alpha1.ObservedResource
}

// KeyFuncIO is the key to the context value where the functionIO pointer is stored
const KeyFuncIO contextKey = iota

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = xkube.SchemeBuilder.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
}

var ErrNotFound = errors.New("not found")

// NewRuntime creates a new Runtime object.
func NewRuntime(ctx context.Context, input []byte) (*Runtime, error) {
	log := controllerruntime.LoggerFrom(ctx)

	log.V(1).Info("Unmarshalling FunctionIO from stdin")
	r := Runtime{}
	err := yaml.Unmarshal(input, &r.io)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal function io: %w", err)
	}
	r.Observed = ObservedResources{
		resources: *observedResources(r.io.Observed.Resources),
		composite: r.io.Observed.Composite,
	}
	r.Desired = DesiredResources{
		resources: *desiredResources(r.io.Desired.Resources),
		composite: r.io.Desired.Composite,
	}

	r.Config, err = parseCompFuncConfig(&r)

	return &r, err
}

func getKubeObjectFrom(ctx context.Context, resources *[]Resource, kon string) (*xkube.Object, error) {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("Getting kube object from resources", "name", kon)
	ko := &xkube.Object{
		TypeMeta: metav1.TypeMeta{
			Kind:       xkube.ObjectKind,
			APIVersion: xkube.ObjectKindAPIVersion,
		},
	}
	err := getFrom(ctx, resources, ko, kon)
	if err != nil {
		return nil, err
	}

	return ko, nil
}

func getFrom(ctx context.Context, resources *[]Resource, obj client.Object, resName string) error {
	log := controllerruntime.LoggerFrom(ctx)
	gvk := obj.GetObjectKind()

	log.V(1).Info("Searching resource by resource name", "name", resName)
	for _, res := range *resources {
		if res.GetName() == resName {
			err := yaml.Unmarshal(res.GetRaw(), obj)
			if err != nil {
				return fmt.Errorf("cannot unmarshal desired resource: %w", err)
			}

			// matching by name is not enough, group and kind should match
			ogvk := obj.GetObjectKind()
			if gvk == ogvk {
				return nil
			}
		}
	}

	log.V(1).Info("No resource found", "name", resName)
	return ErrNotFound
}

func desiredResources(dr []xfnv1alpha1.DesiredResource) *[]Resource {
	resources := make([]Resource, len(dr))

	for i := range dr {
		r := desiredResource{DesiredResource: dr[i]}
		resources[i] = &r
	}

	return &resources
}

func observedResources(or []xfnv1alpha1.ObservedResource) *[]Resource {
	resources := make([]Resource, len(or))

	for i := range or {
		resources[i] = observedResource{ObservedResource: or[i]}
	}

	return &resources
}

func updateKubeObject(obj client.Object, ko *xkube.Object) error {
	kind, _, err := s.ObjectKinds(obj)
	if err != nil {
		return fmt.Errorf("cannot get object kinds from %s: %v", obj.GetName(), err)
	}
	obj.GetObjectKind().SetGroupVersionKind(kind[0])

	rawData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("cannot marshall object %s: %v", obj.GetName(), err)
	}
	ko.Spec.ForProvider.Manifest = runtime.RawExtension{Raw: rawData}
	return nil
}

// AddToScheme adds given SchemeBuilder to the Scheme.
func AddToScheme(obj runtime.SchemeBuilder) error {
	return obj.AddToScheme(s)
}

func parseCompFuncConfig(iof *Runtime) (*corev1.ConfigMap, error) {

	cm := &corev1.ConfigMap{}

	if iof.io.Config != nil {
		return cm, yaml.Unmarshal(iof.io.Config.Raw, cm)
	}

	return cm, nil
}

func (r *Runtime) GetRawFuncIO() *xfnv1alpha1.FunctionIO {
	return &r.io
}

// Small function to help us retrieve bool values from configMap
func (r *Runtime) GetBoolFromConfigMap(key string) bool {
	en, ok := r.Config.Data[key]
	if !ok {
		return false
	}
	enabled, err := strconv.ParseBool(en)
	if err != nil {
		return false
	}
	return enabled
}
