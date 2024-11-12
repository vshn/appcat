package vshnnextcloud

import (
	"context"
	"crypto/rand"
	"crypto/rsa"

	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"

	valid "github.com/asaskevich/govalidator"
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	watchOnlySecretObjectName = "-collabora-code-wo-secret"
	ingressCertificateName    = "-collabora-code-ingress-certificate"
	labelMap                  = map[string]string{runtime.WebhookAllowDeletionLabel: "true"}
)

//go:embed files/coolwsd.xml
var coolwsd string

//go:embed files/coolkit.xml
var coolkit string

func DeployCollabora(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewWarningResult("Cannot GetObservedComposite: " + err.Error())
	}

	if !comp.Spec.Parameters.Service.Collabora.Enabled {
		return runtime.NewNormalResult("Collabora not enabled")
	}

	if !valid.IsDNSName(comp.Spec.Parameters.Service.Collabora.FQDN) {
		return runtime.NewWarningResult("Collabora FQDN is not a valid DNS name: " + comp.Spec.Parameters.Service.Collabora.FQDN)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "pods/log", "pods/status", "pods/attach", "pods/exec"},
			Verbs:     []string{"get", "list", "watch", "create", "watch", "patch", "update", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets"},
			Verbs:     []string{"get", "delete", "watch", "list", "patch", "update", "create"},
		},
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{"appcat-collabora"},
			Verbs:         []string{"use"},
		})
	}

	err = common.AddSaWithRole(ctx, svc, rules, comp.GetName(), comp.GetInstanceNamespace(), "collabora-code", true)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora SA with Role: " + err.Error())
	}

	err = createCoolWSDConfigMap(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora CoolWSD ConfigMap: " + err.Error())
	}

	err = createIssuer(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora certificate Issuer: " + err.Error())
	}

	err = createCertificate(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora Certificate: " + err.Error())
	}

	err = observeCertManagedCertificate(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora watch-only Object: " + err.Error())
	}

	err = createSecretWithRSAKeys(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora RSA keys: " + err.Error())
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		svc.Log.Info("Creating Collabora Secret for Openshift Route")
		err = createSecretForOpenshiftRoute(comp, svc)
		if err != nil {
			svc.Log.Error(err, "Failed to create Collabora Openshift Secret "+err.Error())
			return runtime.NewWarningResult("Failed to add Collabora Openshift Route: " + err.Error())
		}
	}

	svc.Log.Info("Waiting for Nextcloud release to be ready")

	releaseExists := svc.WaitForObservedDependencies(comp.GetName(), comp.GetName()+"-release")
	if !releaseExists {
		return runtime.NewWarningResult("Nextcloud instance not yet ready")
	}

	svc.Log.Info("Nextcloud release is ready, creating STS, Service, Ingress, Issuer, Certificate, ConfigMap, Job")

	err = AddCollaboraSts(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora STS: " + err.Error())
	}

	err = AddCollaboraService(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora Service: " + err.Error())
	}

	err = AddCollaboraIngress(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora Ingress: " + err.Error())
	}

	err = createInstallCollaboraJob(comp, svc)
	if err != nil {
		return runtime.NewWarningResult("Failed to add Collabora install Job: " + err.Error())
	}

	return runtime.NewNormalResult("Collabora deployed and configured!")
}

func AddCollaboraSts(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	sts := &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-collabora-code",
			},
		},
		Spec: v1.StatefulSetSpec{
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
					ServiceAccountName: "sa-collabora-code",
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "DONT_GEN_SSL_CERT",
									Value: "true",
								},
								{
									// https://github.com/nextcloud/all-in-one/discussions/2314
									// https://github.com/CollaboraOnline/online/issues/3102
									Name:  "extra_params",
									Value: "--o:net.proto=IPv4",
								},
							},
							Image: svc.Config.Data["collabora_image"],
							Name:  comp.GetName() + "-collabora-code",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9980,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(svc.Config.Data["collaboraCPURequests"]),
									corev1.ResourceMemory: resource.MustParse(svc.Config.Data["collaboraMemoryRequests"]),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(svc.Config.Data["collaboraCPULimit"]),
									corev1.ResourceMemory: resource.MustParse(svc.Config.Data["collaboraMemoryLimit"]),
								},
							},

							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										// https://help.nextcloud.com/t/nextcloud-office-could-not-establish-connection-to-the-collabora-online-server/185721/8
										"FOWNER",
										"SYS_CHROOT",
										"CHOWN",
										// https://github.com/chrisingenhaag/helm/blob/main/charts/collabora-code/values.yaml#L64
										"MKNOD",
									},
								},
								// https://github.com/CollaboraOnline/online/blob/master/docker/from-packages/Dockerfile#L138
								RunAsUser: ptr.To[int64](100),
							},

							// Mount certificates
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      comp.GetName() + "-collabora-code-tls",
									MountPath: "/etc/coolwsd/cert.pem",
									SubPath:   "tls.crt",
								},
								{
									Name:      comp.GetName() + "-collabora-code-tls",
									MountPath: "/etc/coolwsd/key.pem",
									SubPath:   "tls.key",
								},
								{
									Name:      comp.GetName() + "-collabora-code-tls",
									MountPath: "/etc/coolwsd/ca-chain.cert.pem",
									SubPath:   "ca.crt",
								},
								{
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/coolwsd.xml",
									SubPath:   "coolwsd.xml",
								},
								{
									// need to attach it, it's probably generated by coolwsd
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/coolkitconfig.xcu",
									SubPath:   "coolkit.xml",
								},
								{
									// it reports an error if this file is missing
									// it's necessary to support WOPI protocol: https://learn.microsoft.com/en-us/microsoft-365/cloud-storage-partner-program/rest/
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/proof_key",
									SubPath:   "id_rsa",
								},
								{
									// it creates chroots there
									Name:      "child-roots",
									MountPath: "/opt/cool/child-roots/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "child-roots",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: comp.GetName() + "-collabora-code-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: comp.GetName() + "-collabora-code-tls",
								},
							},
						},
						{
							Name: "coolwsd-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: comp.GetName() + "-collabora-code-coolwsd-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(sts, comp.GetName()+"-collabora-code-sts", runtime.KubeOptionAddLabels(labelMap))
}
func AddCollaboraService(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-collabora-code",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": comp.GetName() + "-collabora-code"},
			Ports: []corev1.ServicePort{
				{
					Port:       9980,
					TargetPort: intstr.FromInt(9980),
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(service, comp.GetName()+"-collabora-code-service", runtime.KubeOptionAddLabels(labelMap))

}
func AddCollaboraIngress(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	annotations := map[string]string{}
	if svc.Config.Data["ingress_annotations"] != "" {

		err := yaml.Unmarshal([]byte(svc.Config.Data["ingress_annotations"]), annotations)
		if err != nil {
			svc.Log.Error(err, "cannot unmarshal ingress annotations from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal ingress annotations from input: %s", err)))
		}
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		annotations["route.openshift.io/termination"] = "reencrypt"
		annotations["route.openshift.io/destination-ca-certificate-secret"] = comp.GetName() + ingressCertificateName
		annotations["haproxy.router.openshift.io/timeout"] = "120s"
		annotations["haproxy.router.openshift.io/hsts_header"] = "max-age=31536000;preload"
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-collabora-code",
			},
			Annotations: annotations,
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
					SecretName: comp.GetName() + "-collabora-code-tls",
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(ingress, comp.GetName()+"-collabora-code-ingress", runtime.KubeOptionAddLabels(labelMap))
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

	return svc.SetDesiredKubeObject(issuer, comp.GetName()+"-collabora-code-issuer", runtime.KubeOptionAddLabels(labelMap))
}

func createCertificate(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	certificate := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-certificate",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.CertificateSpec{
			SecretName: comp.GetName() + "-collabora-code-tls",
			DNSNames:   []string{comp.Spec.Parameters.Service.Collabora.FQDN},
			IssuerRef: certmgrv1.ObjectReference{
				Name: comp.GetName() + "-collabora-code-issuer",
			},
		},
	}

	return svc.SetDesiredKubeObject(certificate, comp.GetName()+"-collabora-code-certificate", runtime.KubeOptionAddLabels(labelMap))
}

func createCoolWSDConfigMap(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-coolwsd-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"coolwsd.xml": coolwsd,
			"coolkit.xml": coolkit,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-collabora-code-coolwsd-config", runtime.KubeOptionAddLabels(labelMap))
}

/*
Generate RSA keys for Collabora Code
necessary for full WOPI support, please refer to: https://learn.microsoft.com/en-us/microsoft-365/cloud-storage-partner-program/rest/
*/
func createSecretWithRSAKeys(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	objName := comp.GetName() + "-collabora-code-rsa-keypair"

	currentSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objName,
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	err := svc.GetObservedKubeObject(currentSecret, objName)
	if err == runtime.ErrNotFound {
		/*
		 this part might look weird, but it's necessary because:
		 - we don't want to overwrite the secret if it already exists
		 - RSA generation is really slow
		*/
		return svc.SetDesiredKubeObject(currentSecret, objName, runtime.KubeOptionAddLabels(labelMap))
	} else if err != nil {
		return err
	}

	privkey, pubkey, err := generateSSHRSAKey()
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"id_rsa":     []byte(privkey),
			"id_rsa.pub": []byte(pubkey),
		},
	}

	return svc.SetDesiredKubeObject(secret, objName, runtime.KubeOptionAddLabels(labelMap))
}

func observeCertManagedCertificate(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	/*
		I'm creating here watch-only object, so it's possible for me get Secret{} values from it without
		using kube client to fetch them. I can't use directly our svc.GetObservedKubeObject() because
		this secret is managed by certmanager.
		createSecretForOpenshiftRoute() will copy values from this secret to new one for Openshift Route
		as it requires different format... ¯\_(ツ)_/¯
	*/
	wo := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-tls",
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	return svc.SetDesiredKubeObject(wo, comp.GetName()+watchOnlySecretObjectName, runtime.KubeOptionAddLabels(labelMap), runtime.KubeOptionObserve)
}

func createSecretForOpenshiftRoute(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	secretFromWatchOnly := &corev1.Secret{}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + ingressCertificateName,
			Namespace: comp.GetInstanceNamespace(),
			Labels:    map[string]string{"app": comp.GetName() + "-collabora-code"},
		},
		Data: map[string][]byte{},
	}

	err := svc.GetObservedKubeObject(secretFromWatchOnly, comp.GetName()+watchOnlySecretObjectName)
	if err != nil {
		svc.Log.Error(err, "Failed to get watch-only secret form collabora.createSecretForOpenshiftRoute()")
		return err
	}

	secret.Data["tls.crt"] = secretFromWatchOnly.Data["ca.crt"]

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"collabora-code-ingress-certificate", runtime.KubeOptionAddLabels(labelMap))
}

func createInstallCollaboraJob(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-install-collabora",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-collabora-code",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](100000),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: comp.GetName() + "-install-collabora",
					Labels: map[string]string{
						"app": comp.GetName() + "-install-collabora",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: comp.GetName() + "-collabora-code-sa",
					Containers: []corev1.Container{
						{
							Name:  comp.GetName() + "-install-collabora",
							Image: "quay.io/appuio/oc:v4.13",
							Command: []string{
								"bash",
								"-c",
								fmt.Sprintf("oc exec deployments/%s -- /install-collabora.sh %s %s %s %s", comp.GetName(), svc.Config.Data["isOpenshift"], comp.GetInstanceNamespace(), comp.GetName(), comp.Spec.Parameters.Service.Collabora.FQDN),
							},
						},
					},

					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, comp.GetName()+"-collabora-install-job", runtime.KubeOptionAddLabels(labelMap))
}

// ssh-keygen -t rsa -N "" -m PEM
func generateSSHRSAKey() (string, string, error) {
	privkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	privkeyBytes := x509.MarshalPKCS1PrivateKey(privkey)
	privkeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privkeyBytes,
	})

	pubkeyBytes, err := x509.MarshalPKIXPublicKey(&privkey.PublicKey)
	if err != nil {
		return "", "", err
	}

	pubkeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubkeyBytes,
	})

	return string(privkeyPem), string(pubkeyPem), nil
}
