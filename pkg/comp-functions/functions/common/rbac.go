package common

import (
	"context"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddSaWithRole creates a service account with the given policy and binds it to the role.
func AddSaWithRole(ctx context.Context, iof *runtime.Runtime, policies []rbacv1.PolicyRule, compName, namespace, suffix string) error {
	serviceAccountName := compName + "-" + suffix + "-serviceaccount"

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-" + suffix,
			Namespace: namespace,
		},
	}

	err := iof.Desired.PutIntoObject(ctx, sa, serviceAccountName)
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

	err = iof.Desired.PutIntoObject(ctx, role, compName+"-"+suffix+"-role", saReference)
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

	return iof.Desired.PutIntoObject(ctx, roleBinding, compName+"-"+suffix+"-rolebinding", roleReference, saReference)
}
