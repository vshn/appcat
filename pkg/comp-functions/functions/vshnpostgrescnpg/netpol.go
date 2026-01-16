package vshnpostgrescnpg

import (
	"context"
	"fmt"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Create a network policy that permits ingress from the cnpg operator
func createCnpgNetworkPolicy(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *v1.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	// Reference: https://cloudnative-pg.io/documentation/1.20/samples/networkpolicy-example.yaml
	netPol := netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-cnpg-operator",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{
				"Ingress",
			},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cnpg.io/cluster": comp.GetName() + "-cluster",
				},
			},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					From: []netv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kubernetes.io/metadata.name": "syn-cnpg-system",
							},
						},
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app.kubernetes.io/name",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"cloudnative-pg", "plugin-barman-cloud"},
								},
							},
						},
					}},
					Ports: []netv1.NetworkPolicyPort{
						{Port: &intstr.IntOrString{IntVal: 8000}},
						{Port: &intstr.IntOrString{IntVal: 5432}},
					},
				},
			},
		},
	}

	if err := svc.SetDesiredKubeObject(&netPol, comp.GetName()+"-allow-cnpg"); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add cnpg network policy: %w", err).Error())
	}

	return nil
}
