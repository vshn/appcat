package vshnpostgres

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var serviceName = "primary-service"

func AddLoadBalancerIPToConnectionDetails(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	if !iof.GetBootFromCompositionConfig("externalDatabaseConnectionsEnabled") {
		return runtime.NewNormal()
	}

	comp, err := getVSHNPostgreSQL(ctx, iof)

	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}

	k8sservice := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: getInstanceNamespace(comp),
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"role": "master",
			},
			Ports: []v1.ServicePort{
				{
					Name:     "pgport",
					Port:     5432,
					Protocol: v1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "pgport",
					},
				},
			},
		},
	}

	if comp.Spec.Parameters.Network.ServiceType == "LoadBalancer" {
		k8sservice.Spec.Type = v1.ServiceTypeLoadBalancer
	} else {
		k8sservice.Spec.Type = v1.ServiceTypeClusterIP
	}

	if err := iof.Desired.PutIntoObject(ctx, k8sservice, fmt.Sprintf("%s-%s", comp.GetName(), serviceName)); err != nil {
		return runtime.NewFatalErr(ctx, "Cannot put service into function io", err)
	}

	if comp.Spec.Parameters.Network.ServiceType == "ClusterIP" {
		return runtime.NewNormal()
	}

	k8sservice, err = getObservedService(ctx, iof, k8sservice, fmt.Sprintf("%s-%s", comp.GetName(), serviceName))
	if err != nil {
		return runtime.NewWarning(ctx, "Cannot yet get service object")
	}

	if k8sservice.Status.LoadBalancer.Ingress == nil {
		return runtime.NewWarning(ctx, "LoadBalancerIP is not ready yet")
	}

	if err := updateConnectionSecretWithLoadBalancerIP(ctx, iof, k8sservice); err != nil {
		return runtime.NewFatalErr(ctx, "Cannot update connection secret", err)
	}

	return runtime.NewNormal()
}

func getVSHNPostgreSQL(ctx context.Context, iof *runtime.Runtime) (*vshnv1.VSHNPostgreSQL, error) {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, comp)
	return comp, err
}

func getObservedService(ctx context.Context, iof *runtime.Runtime, service *v1.Service, objectName string) (*v1.Service, error) {
	err := iof.Observed.GetFromObject(ctx, service, objectName)
	return service, err
}

func updateConnectionSecretWithLoadBalancerIP(ctx context.Context, iof *runtime.Runtime, service *v1.Service) error {
	s := &v1.Secret{}
	err := iof.Observed.GetFromObject(ctx, s, connectionSecretResourceName)
	if err != nil {
		return err
	}

	ip := service.Status.LoadBalancer.Ingress[0].IP
	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
		Name:  "LOADBALANCER_IP",
		Value: ip,
	})

	return nil
}
