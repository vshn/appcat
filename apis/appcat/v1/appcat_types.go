package v1

import (
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"regexp"
	"strings"

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

// +kubebuilder:object:root=true

// AppCat defines the main object for this API Server
type AppCat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppCatSpec   `json:"spec,omitempty"`
	Status AppCatStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppCatList defines a list of AppCat
type AppCatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AppCat `json:"items"`
}

// AppCatSpec defines the desired state of AppCat
// The desired state of AppCat is dynamically populated from composition's labels
type AppCatSpec map[string]string

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
		Resource: "appcats",
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
	spec := AppCatSpec{}
	if comp.Annotations != nil {
		for k, v := range comp.Annotations {
			if strings.HasPrefix(k, PrefixAppCatKey) {
				index := strings.LastIndex(k, "/")
				spec[makeCamelCase(k[index+1:])] = v
			}
		}
	}

	appcat := &AppCat{
		ObjectMeta: *comp.ObjectMeta.DeepCopy(),
		Spec:       spec,
		Status: AppCatStatus{
			CompositionName: comp.Name,
		},
	}
	appcat.Annotations = nil
	appcat.Labels = nil
	return appcat
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

func init() {
	SchemeBuilder.Register(&AppCat{}, &AppCatList{})
}
