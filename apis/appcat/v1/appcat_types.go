package v1

import (
	"encoding/json"
	"regexp"
	"strings"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// +kubebuilder:rbac:groups="apiextensions.crossplane.io",resources=compositions,verbs=get;list;watch

var (
	// OfferedValue is the label value to identify AppCat services
	OfferedValue = "true"

	// PrefixAppCatKey is the label and annotation prefix for AppCat services in compositions.
	PrefixAppCatKey = "metadata.appcat.vshn.io"

	// OfferedKey is the label key to identify AppCat services
	OfferedKey = PrefixAppCatKey + "/offered"
)

// Resource is the name of this resource in plural form
var Resource = "appcats"

// +kubebuilder:object:root=true

// AppCat defines the main object for this API Server
type AppCat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Details `json:"details,omitempty"`
	Plans   map[string]VSHNPlan `json:"plans,omitempty"`
	Status  AppCatStatus        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppCatList defines a list of AppCat
type AppCatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AppCat `json:"items"`
}

// Details are fields that are dynamically parsed from the annotations on a composition.
type Details map[string]string

// VSHNPlan represents a plan for a VSHN service.
// It ignores the scheduling labels and other internal fields.
type VSHNPlan struct {
	Note string `json:"note,omitempty"`
	// JSize is called JSize because protobuf creates a method Size()
	JSize VSHNSize `json:"size,omitempty"`
}

// VSHNSize describes the aspects of the actual plan.
// This needs to be a separate struct as the protobuf generator can't handle
// embedded struct apparently.
type VSHNSize struct {
	CPU    string `json:"cpu,omitempty"`
	Disk   string `json:"disk,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// AppCat needs to implement the builder resource interface
var _ resource.Object = &AppCat{}

func (in *AppCat) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *AppCat) NamespaceScoped() bool {
	return false
}

func (in *AppCat) New() runtime.Object {
	return &AppCat{}
}

func (in *AppCat) NewList() runtime.Object {
	return &AppCatList{}
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *AppCat) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: Resource,
	}
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *AppCat) IsStorageVersion() bool {
	return true
}

var _ resource.ObjectList = &AppCatList{}

func (in *AppCatList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// AppCatStatus defines the observed state of AppCat
type AppCatStatus struct {
	// CompositionName is the name of the composition
	CompositionName string `json:"compositionName,omitempty"`
}

// NewAppCatFromComposition returns an AppCat based on the given composition
// If the composition does not satisfy one of its rules, the func will return nil
func NewAppCatFromComposition(comp *v1.Composition) *AppCat {
	if comp == nil || comp.Labels == nil || comp.Labels[OfferedKey] != OfferedValue {
		return nil
	}

	appCat := &AppCat{
		ObjectMeta: *comp.ObjectMeta.DeepCopy(),
		Status: AppCatStatus{
			CompositionName: comp.Name,
		},
		Details: Details{},
	}
	appCat.Annotations = nil
	appCat.Labels = nil
	if comp.Annotations != nil {
		for k, v := range comp.Annotations {
			if k == PrefixAppCatKey+"/plans" {
				parsePlansJSON(v, appCat)
				continue
			}
			if strings.HasPrefix(k, PrefixAppCatKey) {
				index := strings.LastIndex(k, "/")
				appCat.Details[makeCamelCase(k[index+1:])] = v
			}
		}
	}

	return appCat
}

// makeCamelCase transforms any string in camel case string
func makeCamelCase(s string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9-]+")
	s = reg.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	slices := strings.FieldsFunc(s, func(c rune) bool { return c == '-' })
	strCamel := slices[0]
	for _, v := range slices[1:] {
		strCamel += cases.Title(language.English).String(v)
	}
	return strCamel
}

func parsePlansJSON(jsonPlans string, spec *AppCat) {
	plans := map[string]VSHNPlan{}

	err := json.Unmarshal([]byte(jsonPlans), &plans)
	if err != nil {
		spec.Plans = map[string]VSHNPlan{
			"Plans are currently not available": {},
		}
		return
	}
	spec.Plans = plans
}
