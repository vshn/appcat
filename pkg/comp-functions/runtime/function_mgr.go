package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"dario.cat/mergo"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha2"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	xpapi "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vshn/appcat/v4/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	serviceRegistry = map[string]Service{}
	// the default provider kubernetes name
	providerConfigRefName = "kubernetes"
	// ErrNotFound is the errur returned, if the requested resource is not in the
	// the given function state (desired,observed).
	ErrNotFound = errors.New("not found")
)

const (
	OwnerKindAnnotation    = "appcat.vshn.io/ownerkind"
	OwnerVersionAnnotation = "appcat.vshn.io/ownerapiversion"
	OwnerGroupAnnotation   = "appcat.vshn.io/ownergroup"
	ProtectedByAnnotation  = "appcat.vshn.io/protectedby"
	ProtectsAnnotation     = "appcat.vshn.io/protects"
)

// Step describes a single change within a service.
// It's essentially what was previously called a TransformFunc.
type Step struct {
	Name    string
	Execute func(context.Context, *ServiceRuntime) *xfnproto.Result
}

// ServiceRuntime holds the state for one given service.
// It keeps track of the changes that each step does.
// The actual response will be assembled at the end.
type ServiceRuntime struct {
	Log    logr.Logger
	req    *fnv1beta1.RunFunctionRequest
	resp   *fnv1beta1.RunFunctionResponse
	Config corev1.ConfigMap
	// Copy of the desired resources from the request. Will be added to the resp
	// once all steps are finished.
	desirdResources map[resource.Name]*resource.DesiredComposed
	// connectionDetails contains all connection details that should get added
	// to the desired composite.
	connectionDetails resource.ConnectionDetails
	results           []*xfnproto.Result
	desiredComposite  *composite.Unstructured
	observedComposite *composite.Unstructured
}

// Service contains all steps necessary to provide the service (except the legacy P+T portion).
type Service struct {
	Steps []Step
}

// Manager manages all services and their steps.
// It also provides a proxy mode to offload any service to another GRPC endpoint.
type Manager struct {
	log       logr.Logger
	proxyMode bool
	fnv1beta1.UnimplementedFunctionRunnerServiceServer
}

// KubeObjectOption defines the type of functional parameters for kubeObjects
type KubeObjectOption func(obj *xkube.Object)

// ComposedResourceOption defines the type of functional parameters for Crossplane
// managed resources
type ComposedResourceOption func(obj xpresource.Managed)

// RegisterService will register a service to the map of all services.
func RegisterService(name string, function Service) {
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
func (m Manager) RunFunction(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {

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

	for _, step := range function.Steps {

		m.log.Info("Running step", "name", step.Name)

		result := step.Execute(ctx, sr)
		if result == nil {
			result = NewNormalResult(fmt.Sprintf("%s step %s result: ran successfully", service, step.Name))
		} else {
			result.Message = fmt.Sprintf("%s step %s result: %s", service, step.Name, result.Message)
		}
		sr.AddResult(result)
	}

	err = sr.addUsages()
	if err != nil {
		return errResp, err
	}

	return sr.GetResponse()
}

func (m *Manager) proxyFunction(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {

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

	grpcReq := &fnv1beta1.RunFunctionRequest{}

	err = json.Unmarshal(jsonReq, grpcReq)
	if err != nil {
		return errResp, fmt.Errorf("cannot unmarshal grpc reques: %w", err)
	}

	rsp, err := fnv1beta1.NewFunctionRunnerServiceClient(con).RunFunction(ctx, grpcReq)
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
func NewServiceRuntime(l logr.Logger, config corev1.ConfigMap, req *fnv1beta1.RunFunctionRequest) (*ServiceRuntime, error) {

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

	// We need the observed composition here, as otherwise the
	// connectionDetails are always empty.
	comp, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return &ServiceRuntime{}, err
	}

	l = l.WithValues(
		"resource", comp.Resource.GetName(),
	)

	if comp.Resource.GetClaimReference() != nil {
		l = l.WithValues(
			"claimNamespace", comp.Resource.GetClaimReference().Namespace,
			"claimName", comp.Resource.GetClaimReference().Name)
	}

	return &ServiceRuntime{
		Log:               l,
		Config:            config,
		req:               req,
		desirdResources:   desiredResources,
		connectionDetails: comp.ConnectionDetails,
		results:           []*xfnproto.Result{},
		desiredComposite:  desiredComposite.Resource,
		observedComposite: observedComposite.Resource,
	}, nil
}

// GetResponse returns the response with all desired resources set.
// This is the raw GRPC response for crossplane.
// If at any time s.SetRespones() was called, then this function will
// return the set response.
func (s *ServiceRuntime) GetResponse() (*fnv1beta1.RunFunctionResponse, error) {

	if s.resp != nil {
		return s.resp, nil
	}

	resp := response.To(s.req, response.DefaultTTL)

	err := s.checkReadiness()
	if err != nil {
		return nil, err
	}

	err = response.SetDesiredComposedResources(resp, s.desirdResources)
	if err != nil {
		return nil, err
	}

	comp, err := request.GetDesiredCompositeResource(s.req)
	if err != nil {
		return nil, err
	}

	comp.ConnectionDetails = s.connectionDetails
	if s.desiredComposite != nil {
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

	for _, opt := range opts {
		opt(obj)
	}

	unstructuredObj, err := composed.From(obj)
	if err != nil {
		return err
	}

	s.desirdResources[resource.Name(name)] = &resource.DesiredComposed{Resource: unstructuredObj}
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
// adds it to the desired composed resources. It takes options to manipulate the resulting kubec object before applying.
func (s *ServiceRuntime) SetDesiredKubeObject(obj client.Object, objectName string, opts ...KubeObjectOption) error {

	kobj, err := s.putIntoObject(false, obj, objectName, objectName)
	if err != nil {
		return err
	}

	for _, o := range opts {
		o(kobj)
	}

	return s.SetDesiredComposedResourceWithName(kobj, objectName)
}

// SetDesiredKubeObjectWithName takes any `runtime.Object`, puts it into a provider-kubernetes Object and then
// adds it to the desired composed resources with the specified resource name.
// This should be used if manipulating objects that are declared in the P+T composition.
func (s *ServiceRuntime) SetDesiredKubeObjectWithName(obj client.Object, objectName, resourceName string, opts ...KubeObjectOption) error {

	kobj, err := s.putIntoObject(false, obj, objectName, resourceName)
	if err != nil {
		return err
	}

	for _, o := range opts {
		o(kobj)
	}

	return s.SetDesiredComposedResourceWithName(kobj, resourceName)
}

// KubeOptionAddRefs adds the given references to the kube object.
func KubeOptionAddRefs(refs ...xkube.Reference) KubeObjectOption {
	return func(obj *xkube.Object) {
		obj.Spec.References = refs
	}
}

// KubeOptionAddConnectionDetails adds the given connection details to the kube object.
// DestNamespace speficies the namespace where the associated secret should be saved.
// The associated secret will have the UID of the parent object as the name.
func KubeOptionAddConnectionDetails(destNamespace string, cd ...xkube.ConnectionDetail) KubeObjectOption {
	return func(obj *xkube.Object) {
		obj.Spec.ConnectionDetails = cd
		obj.Spec.WriteConnectionSecretToReference = &xpv1.SecretReference{
			Name:      obj.GetName() + "-cd",
			Namespace: destNamespace,
		}
	}
}

// KubeOptionObserveCreateUpdate sets the object to only create and update.
// Provider-kubernetes will not delete it.
func KubeOptionObserveCreateUpdate(obj *xkube.Object) {
	obj.Spec.ManagementPolicies = append(obj.Spec.ManagementPolicies, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionObserve)
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

// SetDesiredKubeObserveObject takes any `runtime.Object`, puts it into a provider-kubernetes Object and then
// adds it to the desired composed resources.
func (s *ServiceRuntime) SetDesiredKubeObserveObject(obj client.Object, objectName string, refs ...xkube.Reference) error {

	kobj, err := s.putIntoObject(true, obj, objectName, objectName, refs...)
	if err != nil {
		return err
	}

	return s.SetDesiredComposedResourceWithName(kobj, objectName)
}

// putIntoObject adds or updates the desired resource into its kube object
// It will inject the same labels as any managed resource gets.
func (s *ServiceRuntime) putIntoObject(observeOnly bool, o client.Object, kon, resourceName string, refs ...xkube.Reference) (*xkube.Object, error) {

	s.addOwnerReferenceAnnotation(o, false)

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

	if observeOnly {
		ko.Spec.ManagementPolicies = nil
		ko.Spec.ManagementPolicies = append(ko.Spec.ManagementPolicies, xpv1.ManagementActionObserve)
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

	err = mergo.Merge(&s.desiredComposite.Unstructured, tmp.Unstructured)
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
	object, ok := s.req.Observed.Resources[objectName]
	if !ok {
		return map[string][]byte{}, ErrNotFound
	}

	return object.ConnectionDetails, nil
}

// GetObservedComposedResource returns and unmarshalls the observed object into the given managed resource.
func (s *ServiceRuntime) GetObservedComposedResource(obj xpresource.Managed, name string) error {
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
	if res, ok := s.desirdResources[resource.Name(name)]; ok {
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

// GetDesiredKubeObject returns the object as is on the cluster.
func (s *ServiceRuntime) GetDesiredKubeObject(obj client.Object, name string) error {
	res, ok := s.desirdResources[resource.Name(name)]
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
func (s *ServiceRuntime) GetRequest() *fnv1beta1.RunFunctionRequest {
	return s.req
}

// SetResponse directly sets the response for the service.
// Please only use this if the service has one single step.
func (s *ServiceRuntime) SetResponse(resp *fnv1beta1.RunFunctionResponse) {
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

	desired := s.desirdResources

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

	s.desirdResources = desired

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
	return s.desirdResources
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
	delete(s.desirdResources, resource.Name(name))
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

// WaitForDependencies takes two arguments, the name of the main resource, which should be deployed after the dependencies.
// It also takes a list of names for objects to depend on. It does NOT deploy any objects, but check for their existence.
// If true is returned it is safe to continue with adding your main object to the desired resources.
// If the main resource already exists in the observed state it will always return true.
func (s *ServiceRuntime) WaitForDependencies(mainResource string, dependencies ...string) bool {
	if _, ok := s.req.Observed.Resources[mainResource]; ok {
		return true
	}

	if !s.areResourcesReady(dependencies) {
		return false
	}

	return true
}

// WaitForDependenciesWithConnectionDetails does the same as WaitForDependencies but additionally also checks the given list of fields against the
// available connection details.
// objectCDMap should contain a map where the key is the name of the dependeny and the string slice the necessary connection detail fields.
func (s *ServiceRuntime) WaitForDependenciesWithConnectionDetails(mainResource string, objectCDMap map[string][]string) (bool, error) {
	// If the main resource already exists we're done here
	if _, ok := s.req.Observed.Resources[mainResource]; ok {
		return true, nil
	}

	for dep, cds := range objectCDMap {
		ready := s.WaitForDependencies(mainResource, dep)
		if !ready {
			return false, nil
		}

		cd, err := s.GetObservedComposedResourceConnectionDetails(dep)
		if err != nil {
			return false, err
		}

		for _, field := range cds {
			if _, ok := cd[field]; !ok {
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
	ofUnstructuredRaw := s.desirdResources[resource.Name(of)]
	if ofUnstructuredRaw == nil {
		return ErrNotFound
	}
	ofUnstructured := ofUnstructuredRaw.Resource
	byUnstructuredRaw := s.desirdResources[resource.Name(by)]
	if byUnstructuredRaw == nil {
		return ErrNotFound
	}
	byUnstructured := byUnstructuredRaw.Resource

	name := ofUnstructured.GetName() + "-used-by-" + byUnstructured.GetName()
	ofAPIVersion, ofKind := ofUnstructured.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	byAPIVersion, byKind := byUnstructured.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	usage := &xpapi.Usage{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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

	return s.SetDesiredKubeObject(usage, name)
}

func (s *ServiceRuntime) addUsages() error {
	for resName, resource := range s.desirdResources {
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
