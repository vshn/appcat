package common

import (
	"fmt"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNetworkPolicy creates network policy in the instance namespace to allow other namespaces access to the service
func CreateNetworkPolicy(comp Composite, svc *runtime.ServiceRuntime) error {
	return CustomCreateNetworkPolicy(comp.GetAllowedNamespaces(), comp.GetInstanceNamespace(), comp.GetName(), "", comp.GetAllowAllNamespaces(), svc)
}

// CustomCreateNetworkPolicy creates a more flexible network policy
// Use this method when, for instance, a service needs a sub-service with more refined network policy access
func CustomCreateNetworkPolicy(sourceNS []string, instanceNs, name, kubeName string, allowAll bool, svc *runtime.ServiceRuntime) error {
	netPolPeer := []netv1.NetworkPolicyPeer{}
	if !allowAll {
		for _, ns := range sourceNS {
			peer := netv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": ns,
					},
				},
			}
			netPolPeer = append(netPolPeer, peer)
		}

		// add the SLI exporter namespace
		sliNs := svc.Config.Data["sliNamespace"]
		if sliNs != "" {
			netPolPeer = append(netPolPeer, netv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": sliNs,
					},
				},
			})
		}

		xpNs := svc.Config.Data["crossplaneNamespace"]
		if xpNs != "" {
			netPolPeer = append(netPolPeer, netv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": xpNs,
					},
				},
			})
		}
	}

	netPol := netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instanceNs,
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{
				"Ingress",
			},
			PodSelector: metav1.LabelSelector{},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					From: netPolPeer,
				},
			},
		},
	}

	if kubeName == "" {
		kubeName = name + "-netpol"
	}

	err := svc.SetDesiredKubeObject(&netPol, kubeName)
	if err != nil {
		err = fmt.Errorf("cannot create networkPolicy object: %w", err)
		return err
	}

	return nil
}

// AddLoadbalancerNetpolicy will allow all traffic to the namespace, so that the loabalancer
// connection works as well.
func AddLoadbalancerNetpolicy(svc *runtime.ServiceRuntime, comp InfoGetter) error {
	np := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{},
			},
		},
	}

	err := svc.SetDesiredKubeObject(np, comp.GetName()+"-allow-all")
	if err != nil {
		return fmt.Errorf("cannot deploy allow all network policy: %w", err)
	}

	return nil
}
