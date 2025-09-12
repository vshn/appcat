package vshnpostgres

import (
	"context"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const stackgresCredObserver = "stackgres-creds-observer"

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
func addSchedules(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
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
		return runtime.NewWarningResult(fmt.Errorf("cannot get cluster object: %w", err).Error())
	}

	additionalVars := append(extraEnvVars, []corev1.EnvVar{
		{
			Name:  "REPACK_ENABLED",
			Value: strconv.FormatBool(comp.Spec.Parameters.Service.RepackEnabled),
		},
		{
			Name:  "VACUUM_ENABLED",
			Value: strconv.FormatBool(comp.Spec.Parameters.Service.VacuumEnabled),
		},
	}...)

	err = setPsqlMinorVersion(svc, cluster)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot set the minor version for the PostgreSQL instance: %w", err).Error())
	}

	if cluster.Spec.Configurations.Backups != nil && len(*cluster.Spec.Configurations.Backups) > 0 {
		backups := *cluster.Spec.Configurations.Backups
		backups[0].CronSchedule = ptr.To(comp.GetBackupSchedule())
		cluster.Spec.Configurations.Backups = &backups
	}

	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set cluster object: %w", err))
	}

	err = addStackgresCredentialsObserver(svc, comp, sgNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot observe stackgres credentials: %s", err))
	}

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithRole(maintRolename).
		WithAdditionalClusterRoleBinding(fmt.Sprintf("%s:%s", maintRolename, comp.GetName())).
		WithPolicyRules(policyRules).
		WithExtraEnvs(additionalVars...).
		WithExtraResources(createMaintenanceSecret(instanceNamespace, sgNamespace, comp.GetName()+"-maintenance-secret", comp.GetName())).
		Run(ctx)
}

func createMaintenanceSecret(instanceNamespace, sgNamespace, resourceName, compName string) maintenance.ExtraResource {
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
				Name: compName + "-stackgres-creds-observer",
			},
			FieldPath: ptr.To("status.atProvider.manifest.data"),
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

func setPsqlMinorVersion(svc *runtime.ServiceRuntime, desiredCluster *stackgresv1.SGCluster) error {
	observedCluster := &stackgresv1.SGCluster{}
	err := svc.GetObservedKubeObject(observedCluster, "cluster")
	if errors.IsNotFound(err) {
		// Cluster doesn't exist yet. So let's ignore it here
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot get observed cluster object: %w", err)
	}

	pgVersion := observedCluster.Spec.Postgres.Version

	desiredCluster.Spec.Postgres.Version = pgVersion

	return nil
}

func addStackgresCredentialsObserver(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, sgNamespace string) error {

	stackgresCredentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stackgres-restapi-admin",
			Namespace: sgNamespace,
		},
	}

	err := svc.SetDesiredKubeObject(stackgresCredentials, fmt.Sprintf("%s-%s", comp.GetName(), stackgresCredObserver), runtime.KubeOptionObserve)
	if err != nil {
		return fmt.Errorf("cannot deploy stackgres credentials observer: %w", err)
	}

	return nil
}
