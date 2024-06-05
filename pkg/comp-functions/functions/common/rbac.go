package common

import (
	"context"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha2"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddSaWithRole creates a service account with the given policy and binds it to the role.
func AddSaWithRole(ctx context.Context, svc *runtime.ServiceRuntime, policies []rbacv1.PolicyRule, compName, namespace, suffix string) error {
	serviceAccountName := compName + "-" + suffix + "-serviceaccount"

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-" + suffix,
			Namespace: namespace,
		},
	}

	err := svc.SetDesiredKubeObject(sa, serviceAccountName)
	if err != nil {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      suffix + "-role",
			Namespace: namespace,
		},
		Rules: policies,
	}

	saReference := xkubev1.Reference{
		DependsOn: &xkubev1.DependsOn{
			Name: serviceAccountName,
		},
	}

	err = svc.SetDesiredKubeObject(role, compName+"-"+suffix+"-role", runtime.KubeOptionAddRefs(saReference))
	if err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      suffix + "-rolebinding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      sa.GetName(),
				Namespace: sa.GetNamespace(),
				Kind:      rbacv1.ServiceAccountKind,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     role.GetName(),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	roleReference := xkubev1.Reference{
		DependsOn: &xkubev1.DependsOn{
			Name: compName + "-" + suffix + "-role",
		},
	}

	return svc.SetDesiredKubeObject(roleBinding, compName+"-"+suffix+"-rolebinding", runtime.KubeOptionAddRefs(roleReference, saReference))
}
