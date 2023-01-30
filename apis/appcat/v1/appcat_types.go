/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// AppCat
// +k8s:openapi-gen=true
type AppCat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppCatSpec   `json:"spec,omitempty"`
	Status AppCatStatus `json:"status,omitempty"`
}

// AppCatList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppCatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AppCat `json:"items"`
}

// AppCatSpec defines the desired state of AppCat
type AppCatSpec struct {
	// ServiceName is the service name of this AppCat
	ServiceName string `json:"displayName,omitempty"`
}

var _ resource.Object = &AppCat{}
var _ resourcestrategy.Validater = &AppCat{}

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

func (in *AppCat) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "api.appcat.vshn.io",
		Version:  "v1",
		Resource: "appcats",
	}
}

func (in *AppCat) IsStorageVersion() bool {
	return true
}

func (in *AppCat) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &AppCatList{}

func (in *AppCatList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// AppCatStatus defines the observed state of AppCat
type AppCatStatus struct {
}

func (in AppCatStatus) SubResourceName() string {
	return "status"
}

// AppCat implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &AppCat{}

func (in *AppCat) GetStatus() resource.StatusSubResource {
	return in.Status
}

// AppCatStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &AppCatStatus{}

func (in AppCatStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*AppCat).Status = in
}
