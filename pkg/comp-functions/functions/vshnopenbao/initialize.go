package vshnopenbao

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed scripts/init_cluster.sh
var initClusterScript string

const (
	initOutputSecretSuffix = "-init-output"
	unsealKeysSecretSuffix = "-unseal-keys"
	observerSuffix         = "-observer"
	initRoleSuffix         = "-init-role"
)

func InitializeCluster(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if !comp.Spec.Parameters.OpenBao.Init.RunInitJob {
		svc.Log.Info("RunInitJob is set to false. Skip initialize process...")
		return nil
	}

	// Always observe secrets so connection details keep flowing to the user's secret.
	err = observeInitConnectionDetails(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot observe init connection details: %w", err))
	}

	err = createInitRBAC(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create init RBAC: %w", err))
	}

	if !comp.Status.InitializationComplete && isInitSecretPopulated(comp, svc) {
		svc.Log.Info("Init secret detected, marking initialization as complete")
		comp.Status.InitializationComplete = true
		if err = svc.SetDesiredCompositeStatus(comp); err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot set initialization status: %w", err))
		}
	}

	return nil
}

func isInitSecretPopulated(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) bool {
	secret := &corev1.Secret{}
	err := svc.GetObservedKubeObject(secret, comp.GetName()+initOutputSecretSuffix+observerSuffix)
	if err != nil {
		return false
	}
	return len(secret.Data["VAULT_TOKEN"]) > 0
}

func createInitRBAC(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	svc.Log.Info("Create RBAC for OpenBao init sidecar")
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()
	roleName := serviceName + initRoleSuffix

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create", "get", "update", "patch"},
			},
		},
	}
	if err := svc.SetDesiredKubeObject(role, roleName); err != nil {
		return fmt.Errorf("cannot set init role: %w", err)
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}
	if err := svc.SetDesiredKubeObject(rb, roleName+"-binding"); err != nil {
		return fmt.Errorf("cannot set init rolebinding: %w", err)
	}

	return nil
}

func observeInitConnectionDetails(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	rootTokenSecretName := serviceName + initOutputSecretSuffix
	unsealKeysSecretName := serviceName + unsealKeysSecretSuffix

	rootTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootTokenSecretName,
			Namespace: ns,
		},
	}
	err := svc.SetDesiredKubeObject(rootTokenSecret, rootTokenSecretName+observerSuffix,
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       rootTokenSecretName,
					FieldPath:  "data.VAULT_ADDR",
				},
				ToConnectionSecretKey: "VAULT_ADDR",
			},
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       rootTokenSecretName,
					FieldPath:  "data.VAULT_TOKEN",
				},
				ToConnectionSecretKey: "VAULT_TOKEN",
			},
		),
	)
	if err != nil {
		return fmt.Errorf("cannot set root token secret observer: %w", err)
	}

	if err = svc.AddObservedConnectionDetails(rootTokenSecretName + observerSuffix); err != nil {
		return fmt.Errorf("cannot add root token connection details: %w", err)
	}

	unsealKeysSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unsealKeysSecretName,
			Namespace: ns,
		},
	}
	err = svc.SetDesiredKubeObject(unsealKeysSecret, unsealKeysSecretName+observerSuffix,
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       unsealKeysSecretName,
					FieldPath:  "data.keys",
				},
				ToConnectionSecretKey: "keys",
			},
		),
	)
	if err != nil {
		return fmt.Errorf("cannot set unseal keys secret observer: %w", err)
	}

	if err = svc.AddObservedConnectionDetails(unsealKeysSecretName + observerSuffix); err != nil {
		return fmt.Errorf("cannot add unseal keys connection details: %w", err)
	}

	return nil
}
