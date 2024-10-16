package vshnnextcloud

import (
	"context"
	"crypto/md5"
	_ "embed"
	"fmt"

	valid "github.com/asaskevich/govalidator"
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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

//go:embed files/coolwsd.xml
var coolwsd string

//go:embed files/coolkit.xml
var coolkit string

//go:embed files/sample-key.rsa
var sampleKey string

//go:embed files/install-collabora.sh
var installCollabora string

func DeployCollabora(ctx context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	if !comp.Spec.Parameters.Service.Collabora.Enabled {
		return runtime.NewNormalResult("Collabora not enabled")
	}

	if !valid.IsDNSName(comp.Spec.Parameters.Service.Collabora.FQDN) {
		return runtime.NewWarningResult("Collabora FQDN is not a valid DNS name")
	}

	// Create service account
	err = createServiceAccount(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Create security context role
	err = createSecurityContextRole(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Create security context role binding
	err = createSecurityContextRoleBinding(comp, svc)
	if err != nil {
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

	// Create watch-only object
	err = createWatchOnlyObject(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// copy from watched certificate to new one
	err = createSecretForOpenshiftRoute(comp, svc)
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

	// Create install Collabora config map
	err = createInstallCollaboraConfigMap(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	// Create install Collabora job
	err = createInstallCollaboraJob(comp, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	return runtime.NewNormalResult("Collabora deployed and configured!")
}

func AddCollaboraDeployment(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	deployment := &v1.StatefulSet{
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
					ServiceAccountName: comp.GetName() + "-collabora-code-sa",
					SecurityContext: &corev1.PodSecurityContext{
						// https://github.com/CollaboraOnline/online/blob/master/docker/from-packages/Dockerfile#L138
						RunAsUser: ptr.To[int64](100),
					},
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
							Image: "collabora/code:24.04.8.1.1",
							//Image: "collabora/code:latest",
							Name: comp.GetName() + "-collabora-code",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9980,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("900m"),
									corev1.ResourceMemory: resource.MustParse("1200Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("900m"),
									corev1.ResourceMemory: resource.MustParse("1200Mi"),
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
								{
									// need to attach it, it's probably generated by coolwsd
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/coolkitconfig.xcu",
									SubPath:   "coolkit.xml",
								},
								{
									// it reports an error if this file is missing
									Name:      "coolwsd-config",
									MountPath: "/etc/coolwsd/proof_key",
									SubPath:   "sample-key.rsa",
								},
								{
									Name:      "tmp",
									MountPath: "/tmp",
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
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "child-roots",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
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
										Name: "collabora-code-coolwsd-config",
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
					Port:       9980,
					TargetPort: intstr.FromInt(9980),
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
				"cert-manager.io/cluster-issuer":                       "letsencrypt-staging",
				"route.openshift.io/termination":                       "reencrypt",
				"route.openshift.io/destination-ca-certificate-secret": comp.GetName() + "-collabora-code-ingress-certificate",
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
					SecretName: comp.GetName() + "-collabora-code-tls",
				},
			},
		},
	}

	if svc.Config.Data["ingress_annotations"] != "" {
		annotations := map[string]string{}

		err := yaml.Unmarshal([]byte(svc.Config.Data["ingress_annotations"]), annotations)
		if err != nil {
			return fmt.Errorf("cannot unmarshal ingress annotations from input: %w", err)
		}

		if val, ok := annotations["cert-manager.io/cluster-issuer"]; ok {
			ingress.Annotations["cert-manager.io/cluster-issuer"] = val
		}
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
			Name:      "collabora-code-coolwsd-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"coolwsd.xml":    coolwsd,
			"coolkit.xml":    coolkit,
			"sample-key.rsa": sampleKey,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-collabora-code-coolwsd-config")
}

func createServiceAccount(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-sa",
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	return svc.SetDesiredKubeObject(sa, comp.GetName()+"-collabora-code-sa")
}

func createSecurityContextRole(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-role",
			Namespace: comp.GetInstanceNamespace(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				// pods
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "pods/status", "pods/attach", "pods/exec"},
				Verbs:     []string{"*"},
			},
			{
				// deployments
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "delete", "watch", "list", "patch", "update", "create"},
			},
		},
	}

	if svc.Config.Data["isOpenshift"] == "true" {
		role.Rules = append(role.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"security.openshift.io"},
			Resources: []string{"securitycontextconstraints"},
			Verbs:     []string{"use"},
		})
	}

	return svc.SetDesiredKubeObject(role, comp.GetName()+"-collabora-code-role")
}

func createSecurityContextRoleBinding(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-role-binding",
			Namespace: comp.GetInstanceNamespace(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      comp.GetName() + "-collabora-code-sa",
				Namespace: comp.GetInstanceNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     comp.GetName() + "-collabora-code-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return svc.SetDesiredKubeObject(roleBinding, comp.GetName()+"-collabora-code-role-binding")
}

func createWatchOnlyObject(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	// Create a watch-only object to trigger a re-deploy
	wo := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "collabora-code-tls",
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	return svc.SetDesiredKubeObserveObject(wo, comp.GetName()+"-watch-only-secret")
}

func createSecretForOpenshiftRoute(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	secretFromWatchOnly := &corev1.Secret{}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-collabora-code-ingress-certificate",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{},
	}

	err := svc.GetObservedKubeObject(secretFromWatchOnly, comp.GetName()+"-watch-only-secret")
	if err != nil {
		return err
	}

	secret.Data["tls.crt"] = secretFromWatchOnly.Data["ca.crt"]

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"collabora-code-ingress-certificate")
}

func createInstallCollaboraConfigMap(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "collabora-install-script",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"install-collabora.sh": installCollabora,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-collabora-install-script")
}

func createInstallCollaboraJob(comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) error {

	// get hash from config, se job will be triggered if script changes

	observedJob := &batchv1.Job{}
	hash := md5.New()

	_, _ = hash.Write([]byte(installCollabora))
	checksum := hash.Sum(nil)

	strChecksum := fmt.Sprintf("%x", checksum)
	err := svc.GetObservedKubeObject(observedJob, comp.GetName()+"-collabora-install-job")
	if err == nil {
		observedHash, ok := observedJob.Labels["script"]
		if ok && (observedHash != strChecksum) {
			// in this place I want to return no job and with next reconciliation I recreate it
			// whole point is, that there are immutable fields everywhere in job and I cannot update it effectively
			return nil
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-install-collabora",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app":    "appcat",
				"script": fmt.Sprintf("%x", checksum),
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
								fmt.Sprintf("/install-collabora.sh %s %s %s %s", svc.Config.Data["isOpenshift"], comp.GetInstanceNamespace(), comp.GetName(), comp.Spec.Parameters.Service.Collabora.FQDN),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "collabora-install-script",
									MountPath: "/install-collabora.sh",
									SubPath:   "install-collabora.sh",
									ReadOnly:  false,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "collabora-install-script",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode: ptr.To[int32](0755),
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "collabora-install-script",
									},
								},
							},
						},
					},

					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, comp.GetName()+"-collabora-install-job")
}
