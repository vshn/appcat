package vshnpostgres

import (
	"context"
	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
						Name: service + maintSecretName,
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
						Name: service + maintSecretName,
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
						Name: service + maintSecretName,
					},
					Key: "clearPassword",
				},
			},
		},
	}
)

// AddMaintenanceJob will add a job to do the maintenance in for the instance
func AddMaintenanceJob(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	instanceNamespace := getInstanceNamespace(comp)
	sgNamespace := iof.Config.Data["sgNamespace"]
	schedule := comp.Spec.Parameters.Maintenance

	return maintenance.New(comp, iof, schedule, instanceNamespace, service).
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
			FieldPath: pointer.String("data"),
		},
		ToFieldPath: pointer.String("data"),
	}

	return maintenance.ExtraResource{
		Name:     resourceName,
		Resource: secret,
		Refs: []xkubev1.Reference{
			ref,
		},
	}
}
