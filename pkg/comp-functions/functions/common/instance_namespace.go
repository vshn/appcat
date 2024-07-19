package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/vshn/appcat/v4/apis/metadata"
	"github.com/vshn/appcat/v4/pkg/common/quotas"
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
	disableBillingCMKey   = "billingDisabled"
)

func BootstrapInstanceNs(ctx context.Context, comp Composite, serviceName, namespaceResName string, svc *runtime.ServiceRuntime) error {
	l := svc.Log

	claimNs := comp.GetClaimNamespace()
	compositionName := comp.GetName()
	instanceNs := comp.GetInstanceNamespace()
	claimName, ok := comp.GetLabels()[claimNameLabel]
	if !ok {
		return errors.New("no claim name available in composite labels")
	}

	l.Info("creating namespace observer for " + serviceName + " claim namespace")
	err := createNamespaceObserver(claimNs, compositionName, svc)
	if err != nil {
		return fmt.Errorf("cannot create namespace observer for claim namespace: %w", err)
	}

	l.Info("Creating namespace for " + serviceName + " instance")
	err = createInstanceNamespace(serviceName, compositionName, claimNs, instanceNs, namespaceResName, claimName, svc)
	if err != nil {
		return fmt.Errorf("cannot create %s namespace: %w", serviceName, err)
	}

	l.Info("Creating rbac rules for " + serviceName + " instance")
	err = createNamespacePermissions(compositionName, instanceNs, namespaceResName, svc)
	if err != nil {
		return fmt.Errorf("cannot create rbac rules for %s instance: %w", serviceName, err)
	}

	l.Info("Creating namespace policy to allow access to " + serviceName + " instance")
	err = CreateNetworkPolicy(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create namespace policy  for %s instance: %w", serviceName, err)
	}

	l.Info("Add instance namespace to status")
	err = setInstanceNamespaceStatus(svc, comp)
	if err != nil {
		return fmt.Errorf("cannot add instance namespace to composite status: %w", err)
	}

	l.Info("Add default quotas to namespace")
	err = addInitialNamespaceQuotas(ctx, svc, namespaceResName)
	if err != nil {
		return fmt.Errorf("canot add default quotas to namespace: %w", err)
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

func createNamespaceObserver(claimNs string, instance string, svc *runtime.ServiceRuntime) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: claimNs,
		},
	}

	return svc.SetDesiredKubeObserveObject(ns, instance+claimNsObserverSuffix)
}

// Create the namespace for the service instance
func createInstanceNamespace(serviceName, compName, claimNamespace, instanceNamespace, namespaceResName, claimName string, svc *runtime.ServiceRuntime) error {

	org, err := getOrg(compName, svc)
	if err != nil {
		svc.Log.Info("cannot get claim namespace")
		svc.AddResult(runtime.NewWarningResult("cannot get claim namespace"))
	}

	mode := "standalone"
	if _, exists := svc.Config.Data["mode"]; exists {
		mode = svc.Config.Data["mode"]
	}

	ns := &corev1.Namespace{

		ObjectMeta: metav1.ObjectMeta{
			Name: instanceNamespace,
			Labels: map[string]string{
				"appcat.vshn.io/servicename":     serviceName + "-" + mode,
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
		svc.Log.Info("no control namespace defined, please make sure that it's defined in the composition inputs")
		svc.AddResult(runtime.NewWarningResult("no control namespace defined, please make sure that it's defined in the composition inputs"))
	}

	disabled, err := isBillingDisabled(controlNS, instanceNamespace, compName, svc)
	if err != nil {
		// we don't return here, otherwise we risk the namespace getting deleted
		svc.Log.Error(err, "cannot determine billing status of the service")
		svc.AddResult(runtime.NewWarningResult("cannot determine billing status of the service"))
	}
	if disabled {
		ns.ObjectMeta.Labels["appuio.io/billing-name"] = ""
	}

	return svc.SetDesiredKubeObjectWithName(ns, instanceNamespace, namespaceResName)
}

func createNamespacePermissions(instance string, instanceNs string, namespaceResName string, svc *runtime.ServiceRuntime) error {
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
			disableBillingCMKey: "true",
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

	if _, ok := cm.Data[disableBillingCMKey]; ok {
		return true, nil
	}

	return false, nil
}

func setInstanceNamespaceStatus(svc *runtime.ServiceRuntime, comp Composite) error {
	comp.SetInstanceNamespaceStatus()

	return svc.SetDesiredCompositeStatus(comp)
}

// addInitialNamespaceQuotas will add the default quotas to a namespace if they are not yet set.
// This function takes the name of the namespace resource as it appears in the functionIO, it then returns the actual
// function that implements the composition function step.
func addInitialNamespaceQuotas(ctx context.Context, svc *runtime.ServiceRuntime, namespaceKon string) error {
	if !svc.GetBoolFromCompositionConfig("quotasEnabled") {
		svc.Log.Info("Quotas disabled, skipping")
		svc.AddResult(runtime.NewNormalResult("Quotas disabled, skipping"))
		return nil
	}

	ns := &corev1.Namespace{}

	err := svc.GetObservedKubeObject(ns, namespaceKon)
	if err != nil {
		if err == runtime.ErrNotFound {
			err = svc.GetDesiredKubeObject(ns, namespaceKon)
			if err != nil {
				return fmt.Errorf("cannot get namespace: %w", err)
			}
			// Make sure we don't touch this, if there's no name in the namespace.
			if ns.GetName() == "" {
				return fmt.Errorf("namespace doesn't yet have a name")
			}
		} else {
			return fmt.Errorf("cannot get namespace: %w", err)
		}
	}

	objectMeta := &metadata.MetadataOnlyObject{}

	err = svc.GetObservedComposite(objectMeta)
	if err != nil {
		return fmt.Errorf("cannot get composite meta: %w", err)
	}

	s, err := utils.FetchSidecarsFromConfig(ctx, svc)
	if err != nil {
		s = &utils.Sidecars{}
	}

	// We only act if either the quotas were missing or the organization label is not on the
	// namespace. Otherwise we ignore updates. This is to prevent any unwanted overwriting.
	if quotas.AddInitalNamespaceQuotas(ctx, ns, s, objectMeta.TypeMeta.Kind) {
		err = svc.SetDesiredKubeObjectWithName(ns, ns.GetName(), namespaceKon)
		if err != nil {
			return fmt.Errorf("cannot save namespace quotas: %w", err)
		}
	}

	return nil
}
