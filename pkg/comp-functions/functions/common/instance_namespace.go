package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	roleBindingName       = "appcat:services:read"
	claimNsObserverSuffix = "-claim-ns-observer"
	claimNameLabel        = "crossplane.io/claim-name"
)

// InstanceNamespaceInfo provides all the necessary information to create
// an instance namespace.
type InstanceNamespaceInfo interface {
	GetName() string
	GetClaimNamespace() string
	GetInstanceNamespace() string
	GetLabels() map[string]string
}

func BootstrapInstanceNs(ctx context.Context, comp InstanceNamespaceInfo, serviceName, namespaceResName string, svc *runtime.ServiceRuntime) error {
	l := svc.Log

	claimNs := comp.GetClaimNamespace()
	compositionName := comp.GetName()
	instanceNs := comp.GetInstanceNamespace()
	claimName, ok := comp.GetLabels()[claimNameLabel]
	if !ok {
		return errors.New("no claim name available in composite labels")
	}

	l.Info("creating namespace observer for " + serviceName + " claim namespace")
	err := createNamespaceObserver(ctx, claimNs, compositionName, svc)
	if err != nil {
		return fmt.Errorf("cannot create namespace observer for claim namespace: %w", err)
	}

	l.Info("Creating namespace for " + serviceName + " instance")
	err = createInstanceNamespace(ctx, serviceName, compositionName, claimNs, instanceNs, namespaceResName, claimName, svc)
	if err != nil {
		return fmt.Errorf("cannot create %s namespace: %w", serviceName, err)
	}

	l.Info("Creating rbac rules for " + serviceName + " instance")
	err = createNamespacePermissions(ctx, compositionName, instanceNs, namespaceResName, svc)
	if err != nil {
		return fmt.Errorf("cannot create rbac rules for %s instance: %w", serviceName, err)
	}

	return nil
}

func getOrg(instance string, svc *runtime.ServiceRuntime) string {
	ns := &corev1.Namespace{}

	err := svc.GetObservedKubeObject(ns, instance+claimNsObserverSuffix)
	if err != nil {
		return ""
	}
	return ns.GetLabels()[utils.OrgLabelName]
}

func createNamespaceObserver(ctx context.Context, claimNs string, instance string, svc *runtime.ServiceRuntime) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimNs,
		},
	}

	return svc.SetDesiredKubeObserveObject(ns, instance+claimNsObserverSuffix)
}

// Create the namespace for the service instance
func createInstanceNamespace(ctx context.Context, serviceName, compName, claimNamespace, instanceNamespace, namespaceResName, claimName string, svc *runtime.ServiceRuntime) error {

	org := getOrg(compName, svc)
	ns := &corev1.Namespace{

		ObjectMeta: metav1.ObjectMeta{
			Name: instanceNamespace,
			Labels: map[string]string{
				"appcat.vshn.io/servicename":     serviceName + "-standalone",
				"appcat.vshn.io/claim-namespace": claimNamespace,
				"appcat.vshn.io/claim-name":      claimName,
				"appuio.io/no-rbac-creation":     "true",
				"appuio.io/billing-name":         "appcat-" + serviceName,
				"appuio.io/organization":         org,
			},
		},
	}

	return svc.SetDesiredKubeObjectWithName(ns, instanceNamespace, namespaceResName)
}

func createNamespacePermissions(ctx context.Context, instance string, instanceNs string, namespaceResName string, svc *runtime.ServiceRuntime) error {
	ns := &corev1.Namespace{}
	err := svc.GetObservedKubeObject(ns, namespaceResName)
	if err != nil {
		if err == runtime.ErrNotFound {
			err = svc.GetDesiredKubeObject(ns, namespaceResName)
			if err != nil {
				return fmt.Errorf("cannot get namespace: %w", err)
			}
		} else {
			return fmt.Errorf("cannot get namespace: %w", err)
		}
	}

	org := getOrg(instance, svc)
	if org == "" {
		return nil
	}

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
	return svc.SetDesiredKubeObjectWithName(roleBinding, instance+"-service-rolebinding", "namespace-permissions")
}
