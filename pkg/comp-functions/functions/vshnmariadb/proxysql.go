package vshnmariadb

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"text/template"

	_ "embed"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	pdbv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

//go:embed files/proxysql.conf.tmpl
var configTmpl string

var (
	labels = map[string]string{
		"app": "proxysql",
	}
)

type proxySQLConfigParams struct {
	CompName       string
	RootPassword   string
	Namespace      string
	TLS            int
	Users          []proxySQLUsers
	MariaDBVersion string
}

type proxySQLUsers struct {
	Name     string
	Password string
}

// AddProxySQL will add a ProxySQL cluster to the service, if instances > 1.
// This function also creates a main service, which should always be used to connect from the outside.
// This service is necessary to seamlessly scale up and down without changing the IP address.
func AddProxySQL(_ context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get mariadb composite: %s", err))
	}

	disableProtection := comp.Status.CurrentInstances != 0 && comp.GetInstances() != comp.Status.CurrentInstances

	if comp.GetInstances() <= 1 && !disableProtection {
		// nothing else to do here
		// instances=0: suspended, no ProxySQL needed
		// instances=1: single instance, no ProxySQL needed
		return nil
	}

	comp.Status.CurrentInstances = comp.GetInstances()

	// ProxySQL expects the certificates to have specific names...
	svc.Log.Info("Copying certificate secret")
	err = copyCertificateSecret(comp, svc, disableProtection)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot rename secret fields: %s", err))
	}

	svc.Log.Info("Creating config for proxySQL")
	configHash, err := createProxySQLConfig(comp, svc, disableProtection)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create config: %s", err))
	}

	svc.Log.Info("Creating headless service for proxySQL")
	err = createProxySQLHeadlessService(comp, svc, disableProtection)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create headless service: %s", err))
	}

	svc.Log.Info("Creating statefulset for proxySQL")
	err = createProxySQLStatefulset(comp, svc, configHash, disableProtection)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create statefulset: %s", err))
	}

	svc.Log.Info("Creating pdb for proxysql")
	err = createProxySQLPDB(comp, svc, disableProtection)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create PDB for ProxySQL: %s", err))
	}
	
	// CurrentReleaseTag is being overriden in other functions 
	if comp.Spec.Parameters.Maintenance.PinImageTag != "" {
		comp.Status.MariaDBVersion = comp.Spec.Parameters.Maintenance.PinImageTag
	}

	svc.Log.Info("Updating composite status")
	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set mariadb composite status: %s", err))
	}

	return nil
}

func createProxySQLConfig(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, disableProtection bool) (string, error) {

	cd := svc.GetConnectionDetails()

	userList, err := getUserList(comp, svc)
	if err != nil {
		return "", err
	}

	tls := 1
	if !comp.Spec.Parameters.TLS.TLSEnabled {
		tls = 0
	}

	confParams := proxySQLConfigParams{
		CompName:       comp.GetName(),
		RootPassword:   string(cd["MARIADB_PASSWORD"]),
		Namespace:      comp.GetInstanceNamespace(),
		Users:          userList,
		TLS:            tls,
		MariaDBVersion: comp.Status.MariaDBVersion,
	}

	var buf bytes.Buffer
	tmpl, err := template.New("ProxySQLConfig").Parse(configTmpl)
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(&buf, confParams)
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxysql-config",
			Namespace: comp.GetInstanceNamespace(),
		},
		// We have to use stringdata here. For some reason, when using data, it
		// won't update the value reliably...
		StringData: map[string]string{
			"proxysql.cnf": buf.String(),
		},
	}

	hash := md5.Sum(buf.Bytes())

	opts := []runtime.KubeObjectOption{}

	if disableProtection {
		opts = append(opts, runtime.KubeOptionAllowDeletion)
	}

	return hex.EncodeToString(hash[:]), svc.SetDesiredKubeObject(secret, comp.GetName()+"-proxySQL-config", opts...)
}

func getUserList(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) ([]proxySQLUsers, error) {
	users := []proxySQLUsers{}
	userMap := map[string]string{}

	for _, access := range comp.Spec.Parameters.Service.Access {
		userCD, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-userpass-" + *access.User)
		if err != nil {
			if err == runtime.ErrNotFound {
				return users, nil
			}
			return users, err
		}
		userMap[*access.User] = string(userCD["userpass"])
	}

	for k, v := range userMap {
		users = append(users, proxySQLUsers{
			Name:     k,
			Password: v,
		})
	}

	// Maps are not sorted and return a random order every time
	// To avoid restarts on every reconcile due to order changes, we sort the slice
	sort.Slice(users, func(i, j int) bool {
		return users[i].Name < users[j].Name
	})

	return users, nil
}

func createProxySQLHeadlessService(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, disableProtection bool) error {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxysqlcluster",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Port: 6032,
					Name: "proxysql-admin",
				},
			},
			Selector: map[string]string{
				"app": "proxysql",
			},
		},
	}

	opts := []runtime.KubeObjectOption{}

	if disableProtection {
		opts = append(opts, runtime.KubeOptionAllowDeletion)
	}

	return svc.SetDesiredKubeObject(&service, comp.GetName()+"-proxysql-headless-service", opts...)
}

func createProxySQLStatefulset(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, configHash string, disableProtection bool) error {

	cpuLimit := svc.Config.Data["proxysqlCPULimit"]
	if cpuLimit == "" {
		cpuLimit = "500m"
	}

	memoryLimit := svc.Config.Data["proxysqlMemoryLimit"]
	if memoryLimit == "" {
		memoryLimit = "256Mi"
	}

	cpuRequests := svc.Config.Data["proxysqlCPURequests"]
	if cpuRequests == "" {
		cpuRequests = "50m"
	}

	memoryRequests := svc.Config.Data["proxysqlMemoryRequests"]
	if memoryRequests == "" {
		memoryRequests = "64Mi"
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxysql",
			Namespace: comp.GetInstanceNamespace(),
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To[int32](2),
			ServiceName: "proxysqlcluster",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"configHash": configHash,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: labels,
										},
										TopologyKey: "kubernetes.io/hostname",
									},
									Weight: 1,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Image: svc.Config.Data["proxysqlImage"],
							Name:  "proxysql",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuLimit),
									corev1.ResourceMemory: resource.MustParse(memoryLimit),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuRequests),
									corev1.ResourceMemory: resource.MustParse(memoryRequests),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "proxysql-config",
									MountPath: "/etc/proxysql.cnf",
									SubPath:   "proxysql.cnf",
								},
								{
									Name:      "proxysql-emptydir",
									MountPath: "/var/lib/proxysql",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 6033,
									Name:          "proxysql-mysql",
								},
								{
									ContainerPort: 6032,
									Name:          "proxysql-admin",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "proxysql-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "proxysql-config",
								},
							},
						},
						{
							Name: "proxysql-emptydir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if comp.Spec.Parameters.TLS.TLSEnabled {
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "tls-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "proxysql-certs",
				},
			},
		})

		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(sts.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "tls-config",
				MountPath: "/var/lib/proxysql/proxysql-ca.pem",
				SubPath:   "proxysql-ca.pem",
			},
			corev1.VolumeMount{
				Name:      "tls-config",
				MountPath: "/var/lib/proxysql/proxysql-cert.pem",
				SubPath:   "proxysql-cert.pem",
			},
			corev1.VolumeMount{
				Name:      "tls-config",
				MountPath: "/var/lib/proxysql/proxysql-key.pem",
				SubPath:   "proxysql-key.pem",
			})
	}

	opts := []runtime.KubeObjectOption{}

	if disableProtection {
		opts = append(opts, runtime.KubeOptionAllowDeletion)
	}

	return svc.SetDesiredKubeObject(sts, comp.GetName()+"-proxysql-sts", opts...)
}

func copyCertificateSecret(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, disableProtection bool) error {
	if !comp.Spec.Parameters.TLS.TLSEnabled {
		return nil
	}

	certs, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-server-cert")
	if err != nil {
		return fmt.Errorf("cannot get certs for proxysql: %w", err)
	}

	// Secret not yet ready
	if _, exists := certs["ca.crt"]; !exists {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxysql-certs",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"proxysql-ca.pem":   certs["ca.crt"],
			"proxysql-cert.pem": certs["tls.crt"],
			"proxysql-key.pem":  certs["tls.key"],
		},
	}

	opts := []runtime.KubeObjectOption{}

	if disableProtection {
		opts = append(opts, runtime.KubeOptionAllowDeletion)
	}

	return svc.SetDesiredKubeObject(secret, comp.GetName()+"-proxysql-specific-certs", opts...)
}

func createProxySQLPDB(comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime, disableProtection bool) error {
	min := intstr.IntOrString{StrVal: "50%", Type: intstr.String}

	pdb := &pdbv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-proxysql-pdb",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: pdbv1.PodDisruptionBudgetSpec{
			MinAvailable: &min,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}

	opts := []runtime.KubeObjectOption{}

	if disableProtection {
		opts = append(opts, runtime.KubeOptionAllowDeletion)
	}

	err := svc.SetDesiredKubeObject(pdb, comp.GetName()+"-proxysql-pdb", opts...)
	if err != nil {
		return err
	}

	return nil
}
