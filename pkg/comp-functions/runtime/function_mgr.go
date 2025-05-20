package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"

	"dario.cat/mergo"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	xpapi "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"github.com/vshn/appcat/v4/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceRegistry = map[string]any{}
	// the default provider kubernetes name
	providerConfigRefName = "kubernetes"
	// ErrNotFound is the errur returned, if the requested resource is not in the
	// the given function state (desired,observed).
	ErrNotFound = errors.New("not found")
	scheme      = pkg.SetupScheme()
)

const (
	OwnerKindAnnotation               = "appcat.vshn.io/ownerkind"
	OwnerVersionAnnotation            = "appcat.vshn.io/ownerapiversion"
	OwnerGroupAnnotation              = "appcat.vshn.io/ownergroup"
	ProtectedByAnnotation             = "appcat.vshn.io/protectedby"
	ProtectsAnnotation                = "appcat.vshn.io/protects"
	EventForwardAnnotation            = "appcat.vshn.io/forward-events-to"
	ProviderConfigLabel               = "appcat.vshn.io/provider-config"
	ProviderConfigIgnoreLabel         = "appcat.vshn.io/ignore-provider-config"
	WebhookAllowDeletionLabel         = "appcat.vshn.io/webhook-allowdeletion"
	IgnoreConnectionDetailsAnnotation = "appcat.vshn.io/ignore-connection-details"

	ResourceReady   ResourceReadiness = ResourceReadiness(resource.ReadyTrue)
	ResourceUnReady ResourceReadiness = ResourceReadiness(resource.ReadyFalse)
)

// Step describes a single change within a service.
// It's essentially what was previously called a TransformFunc.
// We use reflect heavily with this, any changes in the ordering of the fields
// need to be adjusted in the `RunFunction` function.
type Step[T client.Object] struct {
	Name    string
	Execute func(context.Context, T, *ServiceRuntime) *xfnproto.Result
}

// ServiceRuntime holds the state for one given service.
// It keeps track of the changes that each step does.
// The actual response will be assembled at the end.
type ServiceRuntime struct {
	Log    logr.Logger
	req    *fnv1.RunFunctionRequest
	resp   *fnv1.RunFunctionResponse
	Config corev1.ConfigMap
	// Copy of the desired resources from the request. Will be added to the resp
	// once all steps are finished.
	desiredResources map[resource.Name]*resource.DesiredComposed
	// connectionDetails contains all connection details that should get added
	// to the desired composite.
	connectionDetails resource.ConnectionDetails
	results           []*xfnproto.Result
	desiredComposite  *composite.Unstructured
	observedComposite *composite.Unstructured
	gvk               schema.GroupVersionKind
	// kubeOptionTracker will keep track of all kubeOptions that get applied
	// to the same kube object over each single function call.
	// This ensures that we never drop a previously applied kubeOption again
	// if we read the nested manifest from the kubeObject and re-apply it to the
	// desired manifest.
	kubeOptionTracker map[string][]KubeObjectOption
}

// Service contains all steps necessary to provide the service (except the legacy P+T portion).
// We use reflect heavily with this, any changes in the ordering of the fields
// need to be adjusted in the `RunFunction` function.
type Service[T client.Object] struct {
	Steps []Step[T]
}

// Manager manages all services and their steps.
// It also provides a proxy mode to offload any service to another GRPC endpoint.
type Manager struct {
	log       logr.Logger
	proxyMode bool
	fnv1.UnimplementedFunctionRunnerServiceServer
}

// KubeObjectOption defines the type of functional parameters for kubeObjects
type KubeObjectOption func(obj *xkube.Object)

// ComposedResourceOption defines the type of functional parameters for Crossplane
// managed resources
type ComposedResourceOption func(obj xpresource.Managed)

type ResourceReadiness resource.Ready

type ServiceState interface {
	GetDesiredState() any
	SetObservedState(map[resource.Name]resource.ObservedComposed) error
}

// RegisterService will register a service to the map of all services.
func RegisterService[T client.Object](name string, function Service[T]) {
	serviceRegistry[name] = function
}

// NewManager creates a new manager.
func NewManager(log logr.Logger, proxyMode bool) *Manager {
	return &Manager{
		log:       log,
		proxyMode: proxyMode,
	}
}

func init() {
	pkg.AddToScheme(composed.Scheme)
}

// RunFunction implements the crossplane composition function `FunctionRunnerServiceServer` interface.
func (m Manager) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {

	if m.proxyMode {
		return m.proxyFunction(ctx, req)
	}

	m.log.V(1).Info("Function triggered")

	// errResp is only used to return a valid response in case of errors
	errResp := response.To(req, response.DefaultTTL)

	// Get the comp functions input, previously called config.
	config := &corev1.ConfigMap{}
	if err := request.GetInput(req, config); err != nil {
		response.Fatal(errResp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return errResp, err
	}

	// Determine which service should be reconciled.
	service, ok := config.Data["serviceName"]
	if !ok {
		return errResp, fmt.Errorf("composition function input does not contian the name of the service")
	}

	m.log.Info("Running service", "name", service)

	function, found := serviceRegistry[service]
	if !found {
		return errResp, fmt.Errorf("no function found for service: %s", service)
	}

	sr, err := NewServiceRuntime(m.log, *config, req)
	if err != nil {
		return errResp, err
	}

	ctx = controllerruntime.LoggerInto(ctx, sr.Log)

	// Unfortunately Golang's generics are very limitted when it comes to collection types.
	// While it's possible to have generics in collections like maps and slices,
	// they can only take types of the exact instantiation of the generic.
	// So if we'd use a `map[string]Service[client.Object]` we cannot
	// save a `Service[VSHNPostgreSQL]` in it, because the types don't match up.
	// That's why these reflect functions are necessary.
	for _, step := range m.extractSteps(function) {

		stepName := m.getStepName(step)

		m.log.Info("Running step", "name", stepName)

		obj, err := scheme.New(sr.getCleanGVK())
		if err != nil {
			return nil, fmt.Errorf("cannot parse object for step execution: %w", err)
		}

		result := m.executeStep(ctx, obj.(client.Object), sr, step)
		if result == nil {
			result = NewNormalResult(fmt.Sprintf("%s step %s result: ran successfully", service, stepName))
		} else {
			result.Message = fmt.Sprintf("%s step %s result: %s", service, stepName, result.Message)
			sr.Log.Info(result.Message)
		}
		sr.AddResult(result)
	}

	err = sr.addUsages()
	if err != nil {
		return errResp, fmt.Errorf("cannot add usages: %w", err)
	}
	// err = sr.ForwardEvents()
	// if err != nil {
	// 	return errResp, fmt.Errorf("cannot forward events: %w", err)
	// }
	err = sr.deployConnectionDetailsToInstanceNS()
	if err != nil {
		return errResp, fmt.Errorf("cannot deploy connection details: %w", err)
	}

	return sr.GetResponse()
}

func (m *Manager) extractSteps(service any) []any {
	// We get the first and only field of the service struct
	v := reflect.ValueOf(service).Field(0)
	values := make([]any, v.Len())

	for i := 0; i < v.Len(); i++ {
		values[i] = v.Index(i).Interface()
	}
	return values
}

func (m *Manager) getStepName(step any) string {
	v := reflect.ValueOf(step).Field(0)
	if v.Kind() != reflect.String {
		panic(fmt.Sprintf("%s not a string, can't get step name", v.Kind()))
	}

	return v.String()
}

func (m *Manager) executeStep(ctx context.Context, obj client.Object, sr *ServiceRuntime, step any) *xfnproto.Result {
	v := reflect.ValueOf(step).Field(1)
	if v.Kind() != reflect.Func {
		panic(fmt.Sprintf("%s not a function, cannot run step", v.Kind()))
	}
	res := v.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(obj),
		reflect.ValueOf(sr),
	})

	vRes := res[0]
	if vRes.IsNil() {
		return nil
	}

	return vRes.Interface().(*xfnproto.Result)
}

func (m *Manager) proxyFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {

	m.log.Info("Proxying request")

	// errResp is only used to return a valid response in case of errors
	errResp := response.To(req, response.DefaultTTL)

	// Get the comp functions input, previously called config.
	config := &corev1.ConfigMap{}
	if err := request.GetInput(req, config); err != nil {
		response.Fatal(errResp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return errResp, err
	}

	endpoint, ok := config.Data["proxyEndpoint"]
	if !ok {
		return errResp, fmt.Errorf("no proxyEndpoint specified")
	}

	con, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errResp, err
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return errResp, fmt.Errorf("cannot convert request to json for grpc reques: %w", err)
	}

	grpcReq := &fnv1.RunFunctionRequest{}

	err = json.Unmarshal(jsonReq, grpcReq)
	if err != nil {
		return errResp, fmt.Errorf("cannot unmarshal grpc reques: %w", err)
	}

	rsp, err := fnv1.NewFunctionRunnerServiceClient(con).RunFunction(ctx, grpcReq)
	if err != nil {
		return errResp, err
	}

	jsonResp, err := json.Marshal(rsp)
	if err != nil {
		return errResp, fmt.Errorf("cannot marshal response to json: %w", err)
	}

	finalResponse := &xfnproto.RunFunctionResponse{}

	err = json.Unmarshal(jsonResp, finalResponse)
	if err != nil {
		return errResp, fmt.Errorf("cannot unmarshal json response: %w", err)
	}

	return finalResponse, nil
}

// NewServiceRuntime returns a new runtime for a given service.
func NewServiceRuntime(l logr.Logger, config corev1.ConfigMap, req *fnv1.RunFunctionRequest) (*ServiceRuntime, error) {

	desiredResources, err := request.GetDesiredComposedResources(req)
	if err != nil {
		return &ServiceRuntime{}, err
	}

	desiredComposite, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		return nil, err
	}

	observedComposite, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, err
	}

	l = l.WithValues(
		"resource", observedComposite.Resource.GetName(),
	)

	if observedComposite.Resource.GetClaimReference() != nil {
		l = l.WithValues(
			"claimNamespace", observedComposite.Resource.GetClaimReference().Namespace,
			"claimName", observedComposite.Resource.GetClaimReference().Name)
	}

	return &ServiceRuntime{
		Log:               l,
		Config:            config,
		req:               req,
		desiredResources:  desiredResources,
		connectionDetails: desiredComposite.ConnectionDetails,
		results:           []*xfnproto.Result{},
		desiredComposite:  desiredComposite.Resource,
		observedComposite: observedComposite.Resource,
		gvk:               observedComposite.Resource.GetObjectKind().GroupVersionKind(),
		kubeOptionTracker: map[string][]KubeObjectOption{},
	}, nil
}

// GetResponse returns the response with all desired resources set.
// This is the raw GRPC response for crossplane.
// If at any time s.SetRespones() was called, then this function will
// return the set response.
func (s *ServiceRuntime) GetResponse() (*fnv1.RunFunctionResponse, error) {

	if s.resp != nil {
		return s.resp, nil
	}

	resp := response.To(s.req, response.DefaultTTL)

	err := s.checkReadiness()
	if err != nil {
		return nil, err
	}

	err = s.setProviderConfigs()
	if err != nil {
		return nil, err
	}

	err = response.SetDesiredComposedResources(resp, s.desiredResources)
	if err != nil {
		return nil, err
	}

	comp, err := request.GetDesiredCompositeResource(s.req)
	if err != nil {
		return nil, err
	}

	comp.ConnectionDetails = s.connectionDetails
	if s.desiredComposite != nil {
		// Spec needs to be nil or empty or we can run into issues where we
		// have slight differences between the claim and the composite definitions.
		err = s.desiredComposite.SetValue("spec", nil)
		if err != nil {
			return nil, err
		}
		comp.Resource = s.desiredComposite
	}

	err = response.SetDesiredCompositeResource(resp, comp)

	resp.Results = append(resp.Results, s.results...)

	return resp, err
}

// SetDesiredComposedResource adds the given object to the desired resources, it needs to be a proper
// crossplane Managed Resource.
func (s *ServiceRuntime) SetDesiredComposedResource(obj xpresource.Managed, opts ...ComposedResourceOption) error {
	return s.SetDesiredComposedResourceWithName(obj, obj.GetName(), opts...)
}

// SetDesiredComposedResourceWithName adds the given object to the desired resources, it needs to be a proper
// crossplane Managed Resource. Additionally provide a name, if it's not derived from the object name.
// Usually needed for objects that where migrated from P+T compositions with a static name.
// Additionally it injects the claim-name, claim-namespace and the composite name as a label.
func (s *ServiceRuntime) SetDesiredComposedResourceWithName(obj xpresource.Managed, name string, opts ...ComposedResourceOption) error {

	s.addOwnerReferenceAnnotation(obj, true)

	escapeK8sNames(obj)
	name = EscapeDNS1123(name, false)

	for _, opt := range opts {
		opt(obj)
	}

	unstructuredObj, err := composed.From(obj)
	if err != nil {
		return err
	}

	s.desiredResources[resource.Name(name)] = &resource.DesiredComposed{Resource: unstructuredObj}
	return nil
}

// ComposedOptionProtectedBy protects the given resource from deletion as long
// as resName exists.
// resName is the name of the resource in the desired map.
func ComposedOptionProtectedBy(resName string) ComposedResourceOption {
	return func(obj xpresource.Managed) {
		addProtectionAnnotation(resName, ProtectedByAnnotation, obj)
	}
}

// ComposedOptionProtects is the inverse of ProtectedBy. The object with this annotation
// protects the object with resName.
func ComposedOptionProtects(resName string) ComposedResourceOption {
	return func(obj xpresource.Managed) {
		addProtectionAnnotation(resName, ProtectsAnnotation, obj)
	}
}

// SetDesiredKubeObject takes any `runtime.Object`, puts it into a provider-kubernetes Object and then
// adds it to the desired composed resources. It takes options to manipulate the resulting kube object before applying.
func (s *ServiceRuntime) SetDesiredKubeObject(obj client.Object, objectName string, opts ...KubeObjectOption) error {

	kobj, err := s.putIntoObject(obj, objectName, objectName)
	if err != nil {
		return err
	}

	s.kubeOptionTracker[objectName] = append(s.kubeOptionTracker[objectName], opts...)

	for _, o := range s.kubeOptionTracker[objectName] {
		o(kobj)
	}

	return s.SetDesiredComposedResourceWithName(kobj, objectName)
}

// SetDesiredKubeObjectWithName takes any `runtime.Object`, puts it into a provider-kubernetes Object and then
// adds it to the desired composed resources with the specified resource name.
// This should be used if manipulating objects that are declared in the P+T composition.
func (s *ServiceRuntime) SetDesiredKubeObjectWithName(obj client.Object, objectName, resourceName string, opts ...KubeObjectOption) error {

	kobj, err := s.putIntoObject(obj, objectName, resourceName)
	if err != nil {
		return err
	}

	s.kubeOptionTracker[resourceName] = append(s.kubeOptionTracker[resourceName], opts...)

	for _, o := range s.kubeOptionTracker[resourceName] {
		o(kobj)
	}

	return s.SetDesiredComposedResourceWithName(kobj, resourceName)
}

// KubeOptionLabeler adds the given labels to the kube object.
func KubeOptionAddLabels(labels map[string]string) KubeObjectOption {
	return func(obj *xkube.Object) {
		current := obj.GetLabels()
		if current == nil {
			current = map[string]string{}
		}
		for val, key := range labels {
			current[val] = key
		}
		obj.SetLabels(current)
	}
}

// KubeOptionAddRefs adds the given references to the kube object.
func KubeOptionAddRefs(refs ...xkube.Reference) KubeObjectOption {
	return func(obj *xkube.Object) {
		obj.Spec.References = refs
	}
}

// KubeOptionAllowDeletion adds the allow deletion label to the object
func KubeOptionAllowDeletion(obj *xkube.Object) {
	if obj.ObjectMeta.Labels == nil {
		obj.ObjectMeta.Labels = map[string]string{}
	}
	obj.ObjectMeta.Labels[WebhookAllowDeletionLabel] = "true"
}

// KubeOptionAddConnectionDetails adds the given connection details to the kube object.
// DestNamespace speficies the namespace where the associated secret should be saved.
func KubeOptionAddConnectionDetails(destNamespace string, cd ...xkube.ConnectionDetail) KubeObjectOption {
	return func(obj *xkube.Object) {
		objName := EscapeDNS1123(obj.GetName()+"-cd", false)

		obj.Spec.ConnectionDetails = cd
		obj.Spec.WriteConnectionSecretToReference = &xpv1.SecretReference{
			Name:      objName,
			Namespace: destNamespace,
		}
	}
}

// KubeOptionObserveCreateUpdate sets the object to only create and update.
// Provider-kubernetes will not delete it.
func KubeOptionObserveCreateUpdate(obj *xkube.Object) {
	obj.Spec.ManagementPolicies = nil
	obj.Spec.ManagementPolicies = append(obj.Spec.ManagementPolicies, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionObserve)
}

// KubeOptionObserve sets the object to only observe.
func KubeOptionObserve(obj *xkube.Object) {
	obj.Spec.ManagementPolicies = nil
	obj.Spec.ManagementPolicies = append(obj.Spec.ManagementPolicies, xpv1.ManagementActionObserve)
}

// KubeOptionProtectedBy protects the given kube objects from deletion as long
// as resName exists.
// resName is the name of the resource in the desired map.
func KubeOptionProtectedBy(resName string) KubeObjectOption {
	return func(obj *xkube.Object) {
		addProtectionAnnotation(resName, ProtectedByAnnotation, obj)
	}
}

// KubeOptionProtects is the inverse of ProtectedBy. The object with this annotation
// protects the object with resName.
func KubeOptionProtects(resName string) KubeObjectOption {
	return func(obj *xkube.Object) {
		addProtectionAnnotation(resName, ProtectsAnnotation, obj)
	}
}

// KubeOptionDeployOnControlPlane will ensure that the provider-config for provider
// kubernetes is not overwritten so that the object will be deployed to the control plane.
func KubeOptionDeployOnControlPlane(obj *xkube.Object) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[ProviderConfigIgnoreLabel] = "true"
}

func addProtectionAnnotation(resName, protectionType string, obj client.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if annotations[protectionType] == "" {
		annotations[protectionType] = resName
	} else {
		list := strings.Split(annotations[protectionType], ",")
		list = append(list, resName)
		list = removeDuplicate(list)
		annotations[protectionType] = strings.Join(list, ",")
	}
	obj.SetAnnotations(annotations)
}

func removeDuplicate(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func (s *ServiceRuntime) AddLabels(obj client.Object, labels map[string]string) {
	l := obj.GetLabels()
	maps.Copy(l, labels)
	obj.SetLabels(l)
}

// putIntoObject adds or updates the desired resource into its kube object
// It will inject the same labels as any managed resource gets.
func (s *ServiceRuntime) putIntoObject(o client.Object, kon, resourceName string, refs ...xkube.Reference) (*xkube.Object, error) {

	s.addOwnerReferenceAnnotation(o, false)

	escapeK8sNames(o)

	kind, _, err := composed.Scheme.ObjectKinds(o)
	if err != nil {
		return nil, fmt.Errorf("cannot determine object kind, have you registered it in the scheme: %w", err)
	}

	o.GetObjectKind().SetGroupVersionKind(kind[0])

	// Crossplane uses apply to create and update objects.
	// If we pass an object that already has a populated "kubectl.kubernetes.io/last-applied-configuration"
	// annotation, then it will keep growing with each reconcile.
	// So we reset it here to make sure this doesn't happen.
	annotations := o.GetAnnotations()
	if annotations != nil {
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = ""
		o.SetAnnotations(annotations)
	}

	// We check if there's already an object for this resource name.
	// It there's one we take it's spec and add it to the new object we want to apply.
	// This way we don't override anything, if the object contains other changes outside of `Spec.ForProvider.Manifest`.
	tmpKo := &xkube.Object{}
	koSpec := xkube.ObjectSpec{}
	err = s.GetDesiredComposedResourceByName(tmpKo, resourceName)
	if err != nil && err != ErrNotFound {
		return tmpKo, err
	} else if err == ErrNotFound {
		koSpec = xkube.ObjectSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: providerConfigRefName,
				},
			},
		}
	} else if err == nil {
		koSpec = tmpKo.Spec
	}

	ko := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: kon,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       xkube.ObjectKind,
			APIVersion: xkube.ObjectKindAPIVersion,
		},
		Spec: koSpec,
	}

	// Only set the refs if they are actually set.
	if len(refs) > 0 {
		ko.Spec.References = refs
	}

	ko.Spec.ForProvider.Manifest = runtime.RawExtension{Object: o}

	return ko, nil
}

// GetObservedComposite returns the observed composite and unmarshals it into the given object.
func (s *ServiceRuntime) GetObservedComposite(obj client.Object) error {
	comp, err := request.GetObservedCompositeResource(s.req)
	if err != nil {
		return err
	}

	jsonBytes, err := comp.Resource.MarshalJSON()
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonBytes, obj)
}

// SetDesiredCompositeStatus takes the given composite and updates the status accordingly.
// All other fields will not be updated by crossplane.
func (s *ServiceRuntime) SetDesiredCompositeStatus(obj client.Object) error {
	if s.desiredComposite == nil {
		s.desiredComposite = &composite.Unstructured{
			Unstructured: unstructured.Unstructured{},
		}
	}

	jsonString, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	tmp := &composite.Unstructured{}

	err = json.Unmarshal(jsonString, tmp)
	if err != nil {
		return err
	}

	// Mergo won't update fields without override
	err = mergo.Merge(&s.desiredComposite.Unstructured, tmp.Unstructured, mergo.WithOverride)
	if err != nil {
		return err
	}

	// metadata.managedFields needs to be nil.
	s.desiredComposite.SetManagedFields(nil)
	// also no resource references allowed
	s.desiredComposite.SetResourceReferences(nil)

	return nil
}

// SetConnectionDetail will add the given name/value pair to the map containing all
// desired connection details of the composite. Be careful to not override existing keys.
func (s *ServiceRuntime) SetConnectionDetail(name string, value []byte) {
	s.connectionDetails[name] = value
}

// GetConnectionDetails returns all current connection details for the current
// composite.
func (s *ServiceRuntime) GetConnectionDetails() map[string][]byte {
	return s.connectionDetails
}

// GetObservedComposedResourceConnectionDetails returns the observed connection details of the given
// composed resource.
// Returns an empty map if not found.
func (s *ServiceRuntime) GetObservedComposedResourceConnectionDetails(objectName string) (map[string][]byte, error) {
	objectName = EscapeDNS1123(objectName, false)
	object, ok := s.req.Observed.Resources[objectName]
	if !ok {
		return map[string][]byte{}, ErrNotFound
	}

	return object.ConnectionDetails, nil
}

// GetObservedComposedResource returns and unmarshalls the observed object into the given managed resource.
func (s *ServiceRuntime) GetObservedComposedResource(obj xpresource.Managed, name string) error {
	name = EscapeDNS1123(name, false)
	resources, err := request.GetObservedComposedResources(s.req)
	if err != nil {
		return err
	}
	if res, ok := resources[resource.Name(name)]; ok {
		jsonString, err := res.Resource.Unstructured.MarshalJSON()
		if err != nil {
			return err
		}
		err = json.Unmarshal(jsonString, obj)
		if err != nil {
			return err
		}
		return nil
	}

	return ErrNotFound
}

// GetDesiredComposedResourceByName will return a desired composed resource from the request.
// Use this, if you want anything from a previous function in the pipeline.
func (s *ServiceRuntime) GetDesiredComposedResourceByName(obj xpresource.Managed, name string) error {
	name = EscapeDNS1123(name, false)
	if res, ok := s.desiredResources[resource.Name(name)]; ok {
		jsonString, err := res.Resource.Unstructured.MarshalJSON()
		if err != nil {
			return err
		}
		err = json.Unmarshal(jsonString, obj)
		if err != nil {
			return err
		}
		return nil
	}

	return ErrNotFound
}

// AddResult will add any result the the list of results
func (s *ServiceRuntime) AddResult(result *xfnproto.Result) {
	s.results = append(s.results, result)
}

// NewFatalResult creates a new result with the `FATAL` severity.
// The pipeline will be considdered failed.
func NewFatalResult(err error) *xfnproto.Result {
	return &xfnproto.Result{
		Severity: xfnproto.Severity_SEVERITY_FATAL,
		Message:  err.Error(),
	}
}

// NewWarningResult will return a new warning.
// The pipelines will run through and are not considdered failed.
func NewWarningResult(message string) *xfnproto.Result {
	return &xfnproto.Result{
		Severity: xfnproto.Severity_SEVERITY_WARNING,
		Message:  message,
	}
}

// NewNormalResult creates a new resul with the `NORMAL` severity.
func NewNormalResult(message string) *xfnproto.Result {
	return &xfnproto.Result{
		Severity: xfnproto.Severity_SEVERITY_NORMAL,
		Message:  message,
	}
}

// AddObservedConnectionDetails will add all of the observed connection details of the given
// resouce to the composite's connection details.
func (s *ServiceRuntime) AddObservedConnectionDetails(name string) error {
	cd, err := s.GetObservedComposedResourceConnectionDetails(name)
	if err != nil && err != ErrNotFound {
		return err
	}

	for v, k := range cd {
		s.SetConnectionDetail(v, k)
	}
	return nil
}

// GetObservedKubeObject returns the object as is on the cluster.
func (s *ServiceRuntime) GetObservedKubeObject(obj client.Object, name string) error {
	name = EscapeDNS1123(name, false)
	resources, err := request.GetObservedComposedResources(s.req)
	if err != nil {
		return err
	}

	res, ok := resources[resource.Name(name)]
	if !ok {
		return ErrNotFound
	}

	kube := &xkube.Object{}

	jsonBytes, err := res.Resource.MarshalJSON()
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, kube)
	if err != nil {
		return err
	}

	if len(kube.Status.AtProvider.Manifest.Raw) == 0 {
		return ErrNotFound
	}

	return json.Unmarshal(kube.Status.AtProvider.Manifest.Raw, obj)
}

// GetDesiredKubeObject returns the object as is in the desired state.
func (s *ServiceRuntime) GetDesiredKubeObject(obj client.Object, name string) error {
	name = EscapeDNS1123(name, false)
	res, ok := s.desiredResources[resource.Name(name)]
	if !ok {
		return ErrNotFound
	}

	kube := &xkube.Object{}

	jsonBytes, err := res.Resource.MarshalJSON()
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, kube)
	if err != nil {
		return err
	}

	return json.Unmarshal(kube.Spec.ForProvider.Manifest.Raw, obj)
}

// GetBoolFromCompositionConfig is a small function to help us retrieve bool values from configMap
func (s *ServiceRuntime) GetBoolFromCompositionConfig(key string) bool {
	en, ok := s.Config.Data[key]
	if !ok {
		return false
	}
	enabled, err := strconv.ParseBool(en)
	if err != nil {
		return false
	}
	return enabled
}

// GetRequest returns the pointer to the request.
func (s *ServiceRuntime) GetRequest() *fnv1.RunFunctionRequest {
	return s.req
}

// SetResponse directly sets the response for the service.
// Please only use this if the service has one single step.
func (s *ServiceRuntime) SetResponse(resp *fnv1.RunFunctionResponse) {
	s.resp = resp
}

// checkReadiness checks the readiness of all composed objects.
// As of comp functions beta, we need to make sure that all resources are ready
// by ourselves.
func (s *ServiceRuntime) checkReadiness() error {
	observed, err := request.GetObservedComposedResources(s.req)
	if err != nil {
		return fmt.Errorf("cannot get observed composed resources from %w", err)
	}

	desired := s.desiredResources

	s.Log.V(1).Info("Running readiness check for objects", "count", len(desired))

	// Our goal here is to automatically determine (from the Ready status
	// condition) whether existing composed resources are ready.
	for name, dr := range desired {
		log := s.Log.WithValues("composed-resource-name", name)

		// If this desired resource doesn't exist in the observed resources, it
		// can't be ready because it doesn't yet exist.
		or, ok := observed[name]
		if !ok {
			log.V(1).Info("Ignoring desired resource that does not appear in observed resources")
			s.AddResult(NewWarningResult(fmt.Sprintf("Desire resource is not in observed resources: %s", name)))
			continue
		}

		// A previous Function in the pipeline either said this resource was
		// explicitly ready, or explicitly not ready. We only want to
		// automatically determine readiness for desired resources where no
		// other Function has an opinion about their readiness.
		if dr.Ready != "" && dr.Ready != resource.ReadyUnspecified {
			log.V(1).Info("Ignoring desired resource that already has explicit readiness", "ready", dr.Ready)
			continue
		}

		// Now we know this resource exists, and no Function that ran before us
		// has an opinion about whether it's ready.

		log.V(1).Info("Found desired resource with unknown readiness")
		// If this observed resource has a status condition with type: Ready,
		// status: True, we set its readiness to true.
		c := or.Resource.GetCondition(xpv1.TypeReady)
		if c.Status == corev1.ConditionTrue {
			log.Info("Automatically determined that composed resource is ready")
			dr.Ready = resource.ReadyTrue
		} else {
			log.Info("Composed resource is not ready")
		}
	}

	s.desiredResources = desired

	return nil

}

// GetAllObserved returns a map of all observed resources.
// This is useful when a function needs to have overview about all objects belonging to a service.
func (s *ServiceRuntime) GetAllObserved() (map[resource.Name]resource.ObservedComposed, error) {
	return request.GetObservedComposedResources(s.req)
}

// GetAllDesired returns a map of all observed resources.
// This is useful when a function needs to have overview about all objects belonging to a service.
func (s *ServiceRuntime) GetAllDesired() map[resource.Name]*resource.DesiredComposed {
	return s.desiredResources
}

// SetAllDesired will override the desired resources with the given map.
// WARNING: please only use this if you know what you're doing. This function
// can break the state of any given service!
func (s *ServiceRuntime) SetAllDesired(resources map[resource.Name]*resource.DesiredComposed) {
	s.desiredResources = resources
}

// GetDesiredComposite will return the currently desired composite.
// The only differences from the observed composite will be either in metadata or the status.
// As Crossplane 1.14 composition function forbid any changes other than those fields.
func (s *ServiceRuntime) GetDesiredComposite(obj client.Object) error {

	jsonBytes, err := s.desiredComposite.MarshalJSON()
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonBytes, obj)
}

// DeleteDesiredCompososedResource removes a composite resource from the desired objects.
// If the object is existing on the cluster, it will be deleted!
func (s *ServiceRuntime) DeleteDesiredCompososedResource(name string) {
	delete(s.desiredResources, resource.Name(name))
}

// isResourceSyncedAndReady checks if the given resource is synced and ready.
func (s *ServiceRuntime) isResourceSyncedAndReady(name string) bool {
	obj, ok := s.req.Observed.Resources[name]
	if !ok {
		return false
	}

	unstruct := obj.GetResource().AsMap()

	rawStatus, found, err := unstructured.NestedMap(unstruct, "status")
	if err != nil || !found {
		return false
	}

	status := struct {
		Conditions []xpv1.Condition
	}{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(rawStatus, &status)
	if err != nil {
		return false
	}

	for _, cond := range status.Conditions {
		if cond.Type == xpv1.TypeSynced && cond.Status == "false" {
			return false
		}
		if cond.Type == xpv1.TypeReady && cond.Status == "false" {
			return false
		}
	}

	return true
}

// areResourcesReady checks if all of the given resources are ready or not.
func (s *ServiceRuntime) areResourcesReady(names []string) bool {
	for _, name := range names {
		ok := s.isResourceSyncedAndReady(name)
		if !ok {
			return false
		}
	}
	return true
}

// WaitForObservedDependencies takes two arguments, the name of the main resource, which should be deployed after the dependencies.
// It also takes a list of names for objects to depend on. It does NOT deploy any objects, but check for their existence.
// If true is returned it is safe to continue with adding your main object to the desired resources.
// If the main resource already exists in the observed state it will always return true.
func (s *ServiceRuntime) WaitForObservedDependencies(mainResource string, dependencies ...string) bool {
	if _, ok := s.req.Observed.Resources[mainResource]; ok {
		return true
	}

	if !s.areResourcesReady(dependencies) {
		return false
	}

	return true
}

// WaitForDesiredDependencies takes two arguments, the name of the main resource, which should be deployed after the dependencies.
// It also takes a list of names for objects to depend on. It does NOT deploy any objects, but check for their existence.
// If true is returned it is safe to continue with adding your main object to the desired resources.
// If the main resource already exists in the observed state it will always return true.
func (s *ServiceRuntime) WaitForDesiredDependencies(mainResource string, dependencies ...string) bool {
	if _, ok := s.req.Desired.Resources[mainResource]; ok {
		return true
	}

	if !s.areResourcesReady(dependencies) {
		return false
	}

	return true
}

// WaitForObservedDependenciesWithConnectionDetails does the same as WaitForDependencies but additionally also checks the given list of fields against the
// available connection details. It checks whether the field exists and has a non-empty value.
// objectCDMap should contain a map where the key is the name of the dependeny and the string slice the necessary connection detail fields.
func (s *ServiceRuntime) WaitForObservedDependenciesWithConnectionDetails(mainResource string, objectCDMap map[string][]string) (bool, error) {
	// If the main resource already exists we're done here
	if _, ok := s.req.Observed.Resources[mainResource]; ok {
		return true, nil
	}

	for dep, cds := range objectCDMap {
		ready := s.WaitForObservedDependencies(mainResource, dep)
		if !ready {
			return false, nil
		}

		cd, err := s.GetObservedComposedResourceConnectionDetails(dep)
		if err != nil {
			return false, err
		}

		for _, field := range cds {
			if val, ok := cd[field]; !ok || len(val) == 0 {
				return false, nil
			}
		}
	}

	return true, nil
}

// addOwnerReferenceAnnotation encodes the composite's gvk as a json in the annotations
func (s *ServiceRuntime) addOwnerReferenceAnnotation(obj client.Object, composedResource bool) {
	labels := obj.GetLabels()

	if labels == nil {
		labels = map[string]string{}
	}

	labels[OwnerKindAnnotation] = s.Config.Data["ownerKind"]
	labels[OwnerVersionAnnotation] = s.Config.Data["ownerVersion"]
	labels[OwnerGroupAnnotation] = s.Config.Data["ownerGroup"]

	if !composedResource {
		labels["crossplane.io/composite"] = s.observedComposite.GetLabels()["crossplane.io/composite"]
	}

	obj.SetLabels(labels)
}

// UsageOfBy helps with ordered deletions.
// Sometimes there are objects that are essential for porviders to work.
// For example provider-sql needs secrets to connect to instances.
// During the deletion it's not guaranteed that the secret gets deleted after
// the managed resource that the provider manages.
// This will essentially make it deadlock, as the managed resource will still
// contain a finalizer which blocks the deletion.
// See: https://docs.crossplane.io/latest/concepts/usages/#usage-for-deletion-ordering
//
// Of is the name of the managed resource that should be protected, as set in the desired map
// By is the name of then managed resource which should block the deletion, as set in the desired map. As long as it exists
// the deletion of "Of" will be denied.
func (s *ServiceRuntime) UsageOfBy(of, by string) error {
	ofUnstructuredRaw := s.desiredResources[resource.Name(of)]
	if ofUnstructuredRaw == nil {
		return ErrNotFound
	}
	ofUnstructured := ofUnstructuredRaw.Resource
	byUnstructuredRaw := s.desiredResources[resource.Name(by)]
	if byUnstructuredRaw == nil {
		return ErrNotFound
	}
	byUnstructured := byUnstructuredRaw.Resource

	name := ofUnstructured.GetName() + "-used-by-" + byUnstructured.GetName()
	ofAPIVersion, ofKind := ofUnstructured.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	byAPIVersion, byKind := byUnstructured.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	removed, err := s.unwrapUsage(name)
	if err != nil {
		return fmt.Errorf("cannot remove kube object wrapper from uage: %w", err)
	}
	if !removed {
		return nil
	}

	usage := &xpapi.Usage{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				ProviderConfigIgnoreLabel: "true",
			},
		},
		Spec: xpapi.UsageSpec{
			ReplayDeletion: ptr.To(true),
			Of: xpapi.Resource{
				APIVersion: ofAPIVersion,
				Kind:       ofKind,
				ResourceRef: &xpapi.ResourceRef{
					Name: ofUnstructured.GetName(),
				},
			},
			By: &xpapi.Resource{
				APIVersion: byAPIVersion,
				Kind:       byKind,
				ResourceRef: &xpapi.ResourceRef{
					Name: byUnstructured.GetName(),
				},
			},
		},
	}

	composedRes, err := composed.From(usage)
	if err != nil {
		return fmt.Errorf("cannot convert usage object to managed resource: %w", err)
	}

	s.desiredResources[resource.Name(name)] = &resource.DesiredComposed{Resource: composedRes}

	return nil
}

// unwrapUsage will check if the usage is wrapped into an object.
// It will set the wrapping obect to only observe it. Then on the next reconcile
// after it ensures that the right management policy is set, it will not add it
// to the desired state again. And finally on the third reconcile it will
// return true to indicate that it got removed properly. Allowing Crossplane
// to adopt the Usage objects without wrapping in a kube object.
func (s *ServiceRuntime) unwrapUsage(name string) (bool, error) {
	usage := &xkube.Object{}
	err := s.GetObservedComposedResource(usage, name)
	if err != nil {
		if err == ErrNotFound {
			return true, nil
		}
		return false, err
	}

	resources, err := request.GetObservedComposedResources(s.req)
	if err != nil {
		return false, err
	}
	res := resources[resource.Name(name)]
	if res.Resource.GetKind() != "Object" {
		return true, nil
	}

	// we need to clean the objectmeta, or Crossplane will struggle with adding
	// it to the `resourceRefs` array.
	usage.ObjectMeta = metav1.ObjectMeta{
		Name:            usage.GetName(),
		Annotations:     usage.Annotations,
		Labels:          usage.Labels,
		OwnerReferences: usage.OwnerReferences,
	}

	if len(usage.Spec.ManagementPolicies) == 0 || usage.Spec.ManagementPolicies[0] != xpv1.ManagementActionObserve {
		usage.Spec.ManagementPolicies = xpv1.ManagementPolicies{
			xpv1.ManagementActionObserve,
		}
		err := s.SetDesiredComposedResourceWithName(usage, name)
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (s *ServiceRuntime) addUsages() error {
	for resName, resource := range s.desiredResources {
		byName, protect := resource.Resource.Unstructured.GetAnnotations()[ProtectedByAnnotation]
		if protect {
			resources := strings.Split(byName, ",")
			for _, res := range resources {
				err := s.UsageOfBy(string(resName), res)
				if err != nil {
					if err == ErrNotFound {
						s.Log.Error(err, "cannot add usage for object")
						s.AddResult(NewWarningResult(fmt.Sprintf("cannot add usage for object: %s", err)))
						continue
					}
					return fmt.Errorf("cannot set protected by for object: %w", err)
				}
			}
		}
		ofName, protect := resource.Resource.Unstructured.GetAnnotations()[ProtectsAnnotation]
		if protect {
			resources := strings.Split(ofName, ",")
			for _, res := range resources {
				err := s.UsageOfBy(res, string(resName))
				if err != nil {
					if err == ErrNotFound {
						s.Log.Error(err, "cannot add usage for object")
						s.AddResult(NewWarningResult(fmt.Sprintf("cannot add usage for object: %s", err)))
						continue
					}
					return fmt.Errorf("cannot set protects for object: %w", err)
				}
			}
		}
	}
	return nil
}

func (s *ServiceRuntime) ForwardEvents() error {
	claimRef := s.observedComposite.GetClaimReference()
	// Claim is not yet populated, retry next time
	if claimRef == nil {
		return nil
	}
	eventForwardValue := fmt.Sprintf("%s/%s/%s/%s", claimRef.APIVersion, claimRef.Kind, claimRef.Namespace, claimRef.Name)
	for _, res := range s.desiredResources {
		r := res.Resource

		// For kube objects set the annotation 'EventForwardAnnotation' for themselves and for managed resource
		if isKubeObject(r) {
			p := "spec.forProvider.manifest.metadata.annotations"
			v, _ := r.GetValue(p)
			mrAnnotations := make(map[string]any)
			if v != nil {
				mrAnnotations = v.(map[string]any)
			}
			mrAnnotations[EventForwardAnnotation] = eventForwardValue
			err := r.SetValue(p, mrAnnotations)
			if err != nil {
				return fmt.Errorf("cannot set event forward annotations for managed object %s: %w", r.GetName(), err)
			}
		}
		annotations := r.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[EventForwardAnnotation] = eventForwardValue
		r.SetAnnotations(annotations)
	}
	return nil
}

func isKubeObject(r *composed.Unstructured) bool {
	return r.GetKind() == "Object" && strings.HasPrefix(r.GetAPIVersion(), "kubernetes.crossplane.io")
}

// getCleanGVK will remove the leadin `X` in the Kind.
// Crossplane actuall delivers to us the Composite (XVSHN) object not the Claim
// (VSHN) object. However, in all functions we operate using the Claim objects.
// This leads to a missmatch, when generating the empty object that gets passed to the
// functions.
// This function removes the leading `X` from the Kind to fix this.
func (s *ServiceRuntime) getCleanGVK() schema.GroupVersionKind {
	kind := s.gvk.Kind
	if strings.HasPrefix(kind, "X") {
		kind = kind[1:]
	}

	return schema.GroupVersionKind{
		Group:   s.gvk.Group,
		Version: s.gvk.Version,
		Kind:    kind,
	}
}

// setProviderConfigs loops over all desired objects and adds the providerConfigs
// according to the annotations on the claim/composite.
func (s *ServiceRuntime) setProviderConfigs() error {
	label := ProviderConfigLabel
	if val, exists := s.Config.Data["providerConfigLabel"]; exists && val != "" {
		label = val
	}

	if val, exists := s.observedComposite.GetLabels()[label]; !exists || val == "" || val == "local" {
		return nil
	}

	configName := s.observedComposite.GetLabels()[label]

	for i := range s.desiredResources {
		if _, exists := s.desiredResources[i].Resource.GetLabels()[ProviderConfigIgnoreLabel]; exists {
			continue
		}
		// we set the providerConfig Ref
		err := s.desiredResources[i].Resource.SetString("spec.providerConfigRef.name", configName)
		if err != nil {
			return fmt.Errorf("cannot set providerConfig for %s: %w", s.desiredResources[i].Resource.GetName(), err)
		}

		// We also propagate the label, so if the resource is a composite, then
		// it will automagically also set the right providerConfigs.
		labels := s.desiredResources[i].Resource.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[label] = configName
		s.desiredResources[i].Resource.SetLabels(labels)
	}

	return nil
}

// GetCrossplaneNamespace returns Crossplane's namespace.
// This should mainly be used for `WriteConnectionSecretToRef` in managed resources.
func (s *ServiceRuntime) GetCrossplaneNamespace() string {
	return s.Config.Data["crossplaneNamespace"]
}

// DeployConnectionDetailsToInstanceNS will override the namespace for the connection details to the
// crossplane namespace and also deploy a copy of that to the instance namespace.
// It will prefix the secret name with the composite name, to ensure that no secret names clash in the crossplane namespace.
func (s *ServiceRuntime) deployConnectionDetailsToInstanceNS() error {

	for i := range s.desiredResources {
		cdRef := s.desiredResources[i].Resource.GetWriteConnectionSecretToReference()
		if cdRef == nil {
			continue
		}

		if _, exists := s.desiredResources[i].Resource.GetAnnotations()[IgnoreConnectionDetailsAnnotation]; exists {
			continue
		}

		cd, err := s.GetObservedComposedResourceConnectionDetails(string(i))
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return err
		}

		unstructComp := s.desiredComposite.Unstructured
		instanceNamespace, _, err := unstructured.NestedString(unstructComp.UnstructuredContent(), "status", "instanceNamespace")
		if err != nil {
			return err
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cdRef.Name,
				Namespace: instanceNamespace,
			},
			Data: cd,
		}

		compName := s.desiredResources[i].Resource.GetName()

		// prefix the name in the connetiondetailsref
		// and set to crossplane namespace
		cdRef.Name = compName + "-" + cdRef.Name
		cdRef.Namespace = s.GetCrossplaneNamespace()

		s.desiredResources[i].Resource.SetWriteConnectionSecretToReference(cdRef)

		// We need to concatenate the name of the resource with the name of its immediate parent composite to avoid clashes
		// in the desired map
		err = s.SetDesiredKubeObject(secret, compName+"-"+s.desiredComposite.GetName())
		if err != nil {
			return err
		}
	}

	return nil
}

// SetDesiredResourceReadiness allows us to explicitly set the readiness of any given
// desired resource. This will directly impact the `ready` column in the composite/claim.
func (s *ServiceRuntime) SetDesiredResourceReadiness(name string, ready ResourceReadiness) {
	res := s.desiredResources[resource.Name(name)]
	if res != nil {
		res.Ready = resource.Ready(ready)
		s.desiredResources[resource.Name(name)] = res
	}
}

func (s *ServiceRuntime) ApplyState(state ServiceState) {

	compName := s.observedComposite.GetName()

	desiredState := state.GetDesiredState()

	values := reflect.ValueOf(desiredState)
	typeOfD := values.Type()

	for i := 0; i < values.NumField(); i++ {
		name := strings.ToLower(typeOfD.Field(i).Name)
		iManifest := values.Field(i).Interface()

		if reflect.ValueOf(iManifest).IsNil() {
			s.AddResult(NewFatalResult(fmt.Errorf("one static object is nil, emergency abort! object %s", name)))
			return
		}

		concreteManifest, ok := iManifest.(client.Object)
		if !ok {
			s.AddResult(NewFatalResult(fmt.Errorf("static object not a client.Object: %s", name)))
			return
		}

		cmp, err := composed.From(concreteManifest)
		if err != nil {
			s.AddResult(NewFatalResult(fmt.Errorf("cannot convert object %s to composed resource: %w", name, err)))
			return
		}

		// If it's not a crossplane manage resource we wrap it.
		if _, ok := concreteManifest.(xpresource.Managed); !ok {
			obj, err := s.putIntoObject(concreteManifest, compName+"-"+name, name)
			if err != nil {
				s.AddResult(NewFatalResult(fmt.Errorf("cannot put into kubeObject %s: %w", name, err)))
			}

			tmpCmp, err := composed.From(obj)

			// unstructured.RemoveNestedField(tmpCmp.Object, "status")
			// unstructured.RemoveNestedField(tmpCmp.Object, "spec", "watch")
			// unstructured.RemoveNestedField(tmpCmp.Object, "spec", "forProvider", "manifest", "spec")
			// unstructured.RemoveNestedField(tmpCmp.Object, "spec", "forProvider", "manifest", "status")
			unstructured.RemoveNestedField(tmpCmp.Object, "spec", "forProvider", "manifest", "metadata", "creationTimestamp")

			cmp = tmpCmp
		}

		s.desiredResources[resource.Name(name)] = &resource.DesiredComposed{Resource: cmp}
	}

}

func (s *ServiceRuntime) ObserveState(state ServiceState) {

	allObserved, err := request.GetObservedComposedResources(s.req)
	if err != nil {
		panic(err)
	}

	state.SetObservedState(allObserved)
}
