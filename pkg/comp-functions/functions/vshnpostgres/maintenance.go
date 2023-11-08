package vshnpostgres

import (
	"context"
	"fmt"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	maintSecretName = "maintenancesecret"
	service         = "postgresql"
	maintRolename   = "crossplane:appcat:job:postgres:maintenance"
	policyRules     = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"stackgres.io",
			},
			Resources: []string{
				"sgdbops",
			},
			Verbs: []string{
				"delete",
				"create",
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"stackgres.io",
			},
			Resources: []string{
				"sgclusters",
			},
			Verbs: []string{
				"list",
				"get",
			},
		},
		{
			APIGroups: []string{
				"vshn.appcat.vshn.io",
			},
			Resources: []string{
				"vshnpostgresqls",
			},
			Verbs: []string{
				"get",
				"update",
			},
		},
	}
	extraEnvVars = []corev1.EnvVar{
		{
			Name: "SG_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: maintSecretName,
					},
					Key: "SG_NAMESPACE",
				},
			},
		},
		{
			Name: "API_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: maintSecretName,
					},
					Key: "k8sUsername",
				},
			},
		},
		{
			Name: "API_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: maintSecretName,
					},
					Key: "clearPassword",
				},
			},
		},
	}
)

// addSchedules will add a job to do the maintenance in for the instance
func addSchedules(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	instanceNamespace := getInstanceNamespace(comp)
	sgNamespace := svc.Config.Data["sgNamespace"]
	schedule := comp.GetFullMaintenanceSchedule()

	cluster := &stackgresv1.SGCluster{}
	err = svc.GetDesiredKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get cluster object: %w", err))
	}

	backups := *cluster.Spec.Configurations.Backups
	backups[0].CronSchedule = ptr.To(comp.GetBackupSchedule())
	cluster.Spec.Configurations.Backups = &backups

	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set cluster object: %w", err))
	}

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithRole(maintRolename).
		WithPolicyRules(policyRules).
		WithExtraEnvs(extraEnvVars...).
		WithExtraResources(createMaintenanceSecret(instanceNamespace, sgNamespace, comp.GetName()+"-maintenance-secret")).
		Run(ctx)
}

func createMaintenanceSecret(instanceNamespace, sgNamespace, resourceName string) maintenance.ExtraResource {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintSecretName,
			Namespace: instanceNamespace,
		},
		StringData: map[string]string{
			"SG_NAMESPACE": sgNamespace,
		},
	}

	ref := xkubev1.Reference{
		PatchesFrom: &xkubev1.PatchesFrom{
			DependsOn: xkubev1.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "stackgres-restapi",
				Namespace:  sgNamespace,
			},
			FieldPath: ptr.To("data"),
		},
		ToFieldPath: ptr.To("data"),
	}

	return maintenance.ExtraResource{
		Name:     resourceName,
		Resource: secret,
		Refs: []xkubev1.Reference{
			ref,
		},
	}
}
