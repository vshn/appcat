package vshnnextcloud

import (
	"context"
	"fmt"

	valid "github.com/asaskevich/govalidator"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func DeployCollabora(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if !valid.IsDNSName(comp.Spec.Parameters.Service.Collabora.FQDN) {
		return runtime.NewFatalResult(fmt.Errorf("invalid FQDN"))
	}

	// Add Collabora deployment
	err = AddCollaboraDeployment(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot add Collabora deployment: %w", err))
	}

	// Add Collabora service
	err = AddCollaboraService(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot add Collabora service: %w", err))
	}

	// Add Collabora ingress
	err = AddCollaboraIngress(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot add Collabora ingress: %w", err))
	}

	return runtime.NewNormalResult("Collabora deployed")
}

func AddCollaboraDeployment(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
		},
		Spec: v1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": comp.GetName() + "-collabora-code"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: comp.GetName() + "-collabora-code",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  comp.GetName() + "-collabora-code",
							Image: "collabora/code",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9980,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("1000Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("400Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(deployment, comp.GetName()+"-collabora-code-deployment")
}
func AddCollaboraService(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": comp.GetName() + "-collabora-code"},
			Ports: []corev1.ServicePort{
				{
					Port: 9980,
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(service, comp.GetName()+"-collabora-code-service")

}
func AddCollaboraIngress(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer":              "letsencrypt-staging",
				"nginx.ingress.kubernetes.io/proxy-body-size": "0",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: comp.Spec.Parameters.Service.Collabora.FQDN,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: comp.GetName() + "-collabora-code",
											Port: networkingv1.ServiceBackendPort{
												Number: 9980,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{comp.Spec.Parameters.Service.Collabora.FQDN},
					SecretName: comp.GetName() + "-collabora-code-ingress-cert",
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(ingress, comp.GetName()+"-collabora-code-ingress")

}
