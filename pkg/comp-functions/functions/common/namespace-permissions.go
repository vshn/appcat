package common

import (
	"context"
	"fmt"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const roleBindingName = "appcat:services:read"
const claimNsObserverSuffix = "-claim-ns-observer"

func GetOrg(instance string, svc *runtime.ServiceRuntime) string {
	ns := &corev1.Namespace{}

	err := svc.GetObservedKubeObject(ns, instance+claimNsObserverSuffix)
	if err != nil {
		return ""
	}
	return ns.GetLabels()[utils.OrgLabelName]
}

func CreateNamespaceObserver(ctx context.Context, claimNs string, instance string, svc *runtime.ServiceRuntime) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimNs,
		},
	}

	return svc.SetDesiredKubeObserveObject(ns, instance+claimNsObserverSuffix)
}

func CreateNamespacePermissions(ctx context.Context, instance string, svc *runtime.ServiceRuntime) error {
	ns := &corev1.Namespace{}
	err := svc.GetObservedKubeObject(ns, instance+"-ns")
	if err != nil {
		if err == runtime.ErrNotFound {
			err = svc.GetDesiredKubeObject(ns, instance+"-ns")
			if err != nil {
				return fmt.Errorf("cannot get namespace: %w", err)
			}
		} else {
			return fmt.Errorf("cannot get namespace: %w", err)
		}
	}
	instanceNs := ns.GetName()

	org := ns.GetLabels()[utils.OrgLabelName]

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: instanceNs,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     org,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleBindingName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return svc.SetDesiredKubeObject(roleBinding, instance+"-services-read-rolebinding")
}
