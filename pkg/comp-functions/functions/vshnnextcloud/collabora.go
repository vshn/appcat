package vshnnextcloud

import (
	"context"
	_ "embed"

	valid "github.com/asaskevich/govalidator"
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
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

//go:embed files/coolwsd.xml
var coolwsd string

func DeployCollabora(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	if !comp.Spec.Parameters.Service.Collabora.Enabled {
		return runtime.NewNormalResult("Collabora not enabled")
	}

	if !valid.IsDNSName(comp.Spec.Parameters.Service.Collabora.FQDN) {
		return runtime.NewWarningResult(err.Error())
	}

	// Create coolwsd config map
	err = createCoolWSDConfigMap(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Create issuer
	err = createIssuer(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Create certificate
	err = createCertificate(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Add Collabora deployment
	err = AddCollaboraDeployment(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Add Collabora service
	err = AddCollaboraService(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Add Collabora ingress
	err = AddCollaboraIngress(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	return runtime.NewNormalResult("Collabora deployed")
}

func AddCollaboraDeployment(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
		},
		Spec: v1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": comp.GetName() + "-collabora-code"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   comp.GetName() + "-collabora-code",
					Labels: map[string]string{"app": comp.GetName() + "-collabora-code"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  comp.GetName() + "-collabora-code",
							Image: "collabora/code:24.04.8.1.1",
							Env: []corev1.EnvVar{
								{
									Name:  "DONT_GEN_SSL_CERT",
									Value: "true",
								},
							},
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
							// Mount certificates
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "collabora-code-tls",
									MountPath: "/etc/coolwsd/cert.pem",
									SubPath:   "tls.crt",
								},
								{
									Name:      "collabora-code-tls",
									MountPath: "/etc/coolwsd/key.pem",
									SubPath:   "tls.key",
								},
								{
									Name:      "collabora-code-tls",
									MountPath: "/etc/coolwsd/ca-chain.cert.pem",
									SubPath:   "ca.crt",
								},
								{
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/coolwsd.xml",
									SubPath:   "coolwsd.xml",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "collabora-code-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "collabora-code-tls",
								},
							},
						},
						{
							Name: "coolwsd-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: comp.GetName() + "-collabora-coolwsd-config",
									},
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
func AddCollaboraService(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
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
func AddCollaboraIngress(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
			Annotations: map[string]string{
				// this is the certificate we will use for the ingress
				"cert-manager.io/cluster-issuer": "letsencrypt-staging",
				// collabora is so nice that during startup it generates a self-signed certificate
				// and then uses it for the https connection. This certificate is not trusted by the
				// nginx, so we need to disable ssl verification.
				"nginx.ingress.kubernetes.io/proxy-ssl-verify": "off",
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
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
					Hosts: []string{comp.Spec.Parameters.Service.Collabora.FQDN},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(ingress, comp.GetName()+"-collabora-code-ingress")

}

func createIssuer(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	issuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-issuer",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				SelfSigned: &cmv1.SelfSignedIssuer{},
			},
		},
	}

	return svc.SetDesiredKubeObject(issuer, comp.GetName()+"-collabora-code-issuer")
}

func createCertificate(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	certificate := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-certificate",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.CertificateSpec{
			SecretName: "collabora-code-tls",
			DNSNames:   []string{comp.Spec.Parameters.Service.Collabora.FQDN},
			IssuerRef: certmgrv1.ObjectReference{
				Name: comp.GetName() + "-collabora-code-issuer",
			},
		},
	}

	return svc.SetDesiredKubeObject(certificate, comp.GetName()+"-collabora-code-certificate")
}

func createCoolWSDConfigMap(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "collabora-coolwsd-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"coolwsd.xml": coolwsd,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-collabora-coolwsd-config")
}
