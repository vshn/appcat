package common

import (
	"fmt"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNetworkPolicy creates network policy in the instance namespace to allow other namespaces access to the service
func CreateNetworkPolicy(sourceNs []string, instanceNs string, instance string, svc *runtime.ServiceRuntime) error {
	netPolPeer := []netv1.NetworkPolicyPeer{}
	for _, ns := range sourceNs {
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

	netPol := netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance,
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

	err := svc.SetDesiredKubeObject(&netPol, instance+"-netpol")
	if err != nil {
		err = fmt.Errorf("cannot create networkPolicy object: %w", err)
		return err
	}

	return nil
}
