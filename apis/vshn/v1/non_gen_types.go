package v1

import (
	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:skip
// +kubebuilder:skipclient
// +kubebuilder:skipdeepcopy
// +kubebuilder:object:generate=false
type PodTemplateLabelsManager interface {
	SetPodTemplateLabels(map[string]string)
	GetPodTemplateLabels() map[string]string
	GetObject() client.Object
	client.Object
}

// +kubebuilder:skip
// +kubebuilder:skipclient
// +kubebuilder:skipdeepcopy
// +kubebuilder:object:generate=false
type AddOn interface {
	GetName() string
	GetInstances() int
}

// +kubebuilder:skip
// +kubebuilder:skipclient
// +kubebuilder:skipdeepcopy
// +kubebuilder:object:generate=false
type DeploymentManager struct {
	v1.Deployment
}

// +kubebuilder:skip
// +kubebuilder:skipclient
// +kubebuilder:skipdeepcopy
// +kubebuilder:object:generate=false
type StatefulSetManager struct {
	v1.StatefulSet
}

func (d *DeploymentManager) SetPodTemplateLabels(labels map[string]string) {
	d.Spec.Template.SetLabels(labels)
}

func (s *StatefulSetManager) SetPodTemplateLabels(labels map[string]string) {
	s.Spec.Template.SetLabels(labels)
}

func (d *DeploymentManager) GetPodTemplateLabels() map[string]string {
	return d.Spec.Template.GetLabels()
}

func (s *StatefulSetManager) GetPodTemplateLabels() map[string]string {
	return s.Spec.Template.GetLabels()
}

func (d *DeploymentManager) GetObject() client.Object {
	return &d.Deployment
}

func (d *StatefulSetManager) GetObject() client.Object {
	return &d.StatefulSet
}
