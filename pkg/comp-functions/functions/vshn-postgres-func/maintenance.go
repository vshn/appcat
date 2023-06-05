package vshnpostgres

import (
	"context"
	"fmt"
	"regexp"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	maintServiceAccountName = "maintenanceserviceaccount"
	maintRolename           = "crossplane:appcat:job:postgres:maintenance"
	dayOfWeekMap            = map[string]int{
		"monday":    1,
		"tuesday":   2,
		"wednesday": 3,
		"thursday":  4,
		"friday":    5,
		"saturday":  6,
		"sunday":    0,
	}
	maintSecretName = "maintenancesecret"
)

// AddMaintenanceJob will add a job to do the maintenance in for the instance
func AddMaintenanceJob(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	log.Info("Adding maintenance cronjobs to the instance")
	cron, err := parseCron(comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't parse maintenance to cron", err)
	}
	if cron == "" {
		log.Info("Maintenance schedule not yet populated")
		return runtime.NewNormal()
	}

	err = createMaintenanceServiceAccount(ctx, getInstanceNamespace(comp), comp.GetName(), iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't create maintenance serviceaccount", err)
	}

	err = createMaintenanceRole(ctx, getInstanceNamespace(comp), comp.GetName(), iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't create maintenance role", err)
	}

	err = createMaintenanceRolebinding(ctx, getInstanceNamespace(comp), comp.GetName(), iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't create maintenance rolebinding", err)
	}

	err = createMaintenanceSecret(ctx, getInstanceNamespace(comp), comp.GetName(), iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't create maintenance secret", err)
	}

	err = createMaintenanceJob(ctx, getInstanceNamespace(comp), comp.GetName(), cron, iof, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't create maintenance job", err)
	}

	return runtime.NewNormal()
}

func createMaintenanceServiceAccount(ctx context.Context, instanceNamespace, compositeName string, iof *runtime.Runtime) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintServiceAccountName,
			Namespace: instanceNamespace,
		},
	}

	return iof.Desired.PutIntoObject(ctx, sa, compositeName+"-maintenance-serviceaccount")
}

func createMaintenanceRole(ctx context.Context, instanceNamespace, compositeName string, iof *runtime.Runtime) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintRolename,
			Namespace: instanceNamespace,
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}

	return iof.Desired.PutIntoObject(ctx, role, compositeName+"-maintenance-role")
}

func createMaintenanceRolebinding(ctx context.Context, instanceNamespace, compositeName string, iof *runtime.Runtime) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintRolename,
			Namespace: instanceNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     maintRolename,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: maintServiceAccountName,
			},
		},
	}

	return iof.Desired.PutIntoObject(ctx, roleBinding, compositeName+"-maintenance-rolebinding")
}

func createMaintenanceJob(ctx context.Context, instanceNamespace, compositeName, schedule string, iof *runtime.Runtime, comp *vshnv1.VSHNPostgreSQL) error {

	imageTag := iof.Config.Data["imageTag"]
	if imageTag == "" {
		return fmt.Errorf("no imageTag field in composition function configuration")
	}

	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maintenancejob",
			Namespace: instanceNamespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   schedule,
			SuccessfulJobsHistoryLimit: pointer.Int32(0),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ServiceAccountName: maintServiceAccountName,
							RestartPolicy:      corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "maintenancejob",
									Image: "ghcr.io/vshn/appcat:" + imageTag,
									Env: []corev1.EnvVar{
										{
											Name:  "INSTANCE_NAMESPACE",
											Value: instanceNamespace,
										},
										{
											Name:  "CLAIM_NAME",
											Value: comp.GetLabels()["crossplane.io/claim-name"],
										},
										{
											Name:  "CLAIM_NAMESPACE",
											Value: comp.GetLabels()["crossplane.io/claim-namespace"],
										},
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
									},
									Args: []string{
										"maintenance",
										"--service",
										"postgresql",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return iof.Desired.PutIntoObject(ctx, job, compositeName+"-maintenancejob")
}

func createMaintenanceSecret(ctx context.Context, instanceNamespace, compositeName string, iof *runtime.Runtime) error {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintSecretName,
			Namespace: instanceNamespace,
		},
		StringData: map[string]string{
			"SG_NAMESPACE": iof.Config.Data["sgNamespace"],
		},
	}

	ref := xkubev1.Reference{
		PatchesFrom: &xkubev1.PatchesFrom{
			DependsOn: xkubev1.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "stackgres-restapi",
				Namespace:  iof.Config.Data["sgNamespace"],
			},
			FieldPath: pointer.String("data"),
		},
		ToFieldPath: pointer.String("data"),
	}

	return iof.Desired.PutIntoObject(ctx, secret, compositeName+"-maintenance-secret", ref)
}

func parseCron(comp *vshnv1.VSHNPostgreSQL) (string, error) {

	maintSpec := comp.Spec.Parameters.Maintenance

	if maintSpec.DayOfWeek == "" || maintSpec.TimeOfDay == "" {
		return "", nil
	}

	cronDayOfWeek := dayOfWeekMap[maintSpec.DayOfWeek]

	r := regexp.MustCompile(`(\d+):(\d+):.*`)
	timeSlice := r.FindStringSubmatch(maintSpec.TimeOfDay)

	if len(timeSlice) == 0 {
		return "", fmt.Errorf("Not a valid time string %s", maintSpec.TimeOfDay)
	}

	return fmt.Sprintf("%s %s * * %d", timeSlice[2], timeSlice[1], cronDayOfWeek), nil
}
