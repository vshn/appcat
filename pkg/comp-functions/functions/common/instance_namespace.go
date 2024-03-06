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
	billingCMKey          = "billingDisabled"
)

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

func getOrg(instance string, svc *runtime.ServiceRuntime) (string, error) {
	ns := &corev1.Namespace{}

	err := svc.GetObservedKubeObject(ns, instance+claimNsObserverSuffix)
	if err != nil {
		return "", err
	}
	return ns.GetLabels()[utils.OrgLabelName], nil
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

	org, err := getOrg(compName, svc)
	if err != nil {
		return fmt.Errorf("cannot get claim namespace: %w", err)
	}

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

	controlNS, ok := svc.Config.Data["controlNamespace"]
	if !ok {
		return fmt.Errorf("controlNamespace not specified")
	}

	disabled, err := isBillingDisabled(controlNS, instanceNamespace, compName, svc)
	if err != nil {
		return err
	}
	if disabled {
		ns.ObjectMeta.Labels["appuio.io/billing-name"] = ""
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

	org, err := getOrg(instance, svc)
	if err != nil {
		return fmt.Errorf("cannot get claim namespace: %w", err)
	}

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

// DisableBilling deploys a special config map to the appcat control namespace.
// This configMap contains a key that specifies if a given namespace should be billed or not.
// The configMap can also be used for other configurations in the future.
func DisableBilling(instanceNamespace string, svc *runtime.ServiceRuntime) error {
	controlNS, ok := svc.Config.Data["controlNamespace"]
	if !ok {
		return fmt.Errorf("controlNamespace not specified in composition input")
	}

	objName := instanceNamespace + "-config"

	checkCM := &corev1.ConfigMap{}
	err := svc.GetDesiredKubeObject(checkCM, objName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceNamespace,
			Namespace: controlNS,
		},
		Data: map[string]string{
			billingCMKey: "true",
		},
	}

	err = svc.SetDesiredKubeObject(cm, objName)
	if err != nil {
		return err
	}

	return nil
}

// isBillingDisabled checks the given namespace for the configMap and key that disable billing.
func isBillingDisabled(controlNS, instanceNamespace, compName string, svc *runtime.ServiceRuntime) (bool, error) {

	objSuffix := "-ns-conf-observer"

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceNamespace,
			Namespace: controlNS,
		},
	}

	err := svc.SetDesiredKubeObject(cm, compName+objSuffix)
	if err != nil {
		return false, err
	}

	err = svc.GetObservedKubeObject(cm, compName+objSuffix)
	if err != nil {
		if err == runtime.ErrNotFound {
			return false, nil
		}
		return false, err
	}

	if _, ok := cm.Data[billingCMKey]; ok {
		return true, nil
	}

	return false, nil
}
