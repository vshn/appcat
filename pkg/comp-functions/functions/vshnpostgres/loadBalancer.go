package vshnpostgres

import (
	"context"
	"fmt"

	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var serviceName = "primary-service"

func AddLoadBalancerIPToConnectionDetails(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	if !svc.GetBoolFromCompositionConfig("externalDatabaseConnectionsEnabled") {
		return nil
	}

	comp, err := getVSHNPostgreSQL(ctx, svc)

	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
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

	if err := svc.SetDesiredKubeObject(k8sservice, fmt.Sprintf("%s-%s", comp.GetName(), serviceName)); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot put service into function io: %w", err))
	}

	if comp.Spec.Parameters.Network.ServiceType == "ClusterIP" {
		return nil
	}

	k8sservice, err = getObservedService(ctx, svc, k8sservice, fmt.Sprintf("%s-%s", comp.GetName(), serviceName))
	if err != nil {
		return runtime.NewWarningResult("Cannot yet get service object")
	}

	if k8sservice.Status.LoadBalancer.Ingress == nil {
		return runtime.NewWarningResult("LoadBalancerIP is not ready yet")
	}

	if err := updateConnectionSecretWithLoadBalancerIP(ctx, svc, k8sservice); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot update connection secret: %w", err))
	}

	return nil
}

func getVSHNPostgreSQL(ctx context.Context, svc *runtime.ServiceRuntime) (*vshnv1.VSHNPostgreSQL, error) {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	return comp, err
}

func getObservedService(ctx context.Context, svc *runtime.ServiceRuntime, service *v1.Service, objectName string) (*v1.Service, error) {
	err := svc.GetObservedKubeObject(service, objectName)
	return service, err
}

func updateConnectionSecretWithLoadBalancerIP(ctx context.Context, svc *runtime.ServiceRuntime, service *v1.Service) error {
	s := &v1.Secret{}
	err := svc.GetObservedKubeObject(s, connectionSecretResourceName)
	if err != nil {
		return err
	}

	ip := service.Status.LoadBalancer.Ingress[0].IP
	svc.SetConnectionDetail("LOADBALANCER_IP", []byte(ip))

	return nil
}
