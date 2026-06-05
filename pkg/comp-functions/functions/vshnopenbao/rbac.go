package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serverRoleSuffix = "-server"
	initRoleSuffix   = "-init-role"
)

func ConfigureRBAC(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	if err := configureDiscoveryRBAC(serviceName, ns, svc); err != nil {
		return runtime.NewWarningResult(err.Error())
	}
	if err := configureInitRBAC(serviceName, ns, svc); err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	return nil
}

func configureDiscoveryRBAC(serviceName, ns string, svc *runtime.ServiceRuntime) error {
	roleName := serviceName + serverRoleSuffix

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "watch", "list", "update", "patch"},
			},
		},
	}
	if err := svc.SetDesiredKubeObject(role, roleName); err != nil {
		return fmt.Errorf("cannot add discovery role: %w", err)
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
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
		return fmt.Errorf("cannot add discovery rolebinding: %w", err)
	}

	return nil
}

func configureInitRBAC(serviceName, ns string, svc *runtime.ServiceRuntime) error {
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
		return fmt.Errorf("cannot add init role: %w", err)
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
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
		return fmt.Errorf("cannot add init rolebinding: %w", err)
	}

	return nil
}
