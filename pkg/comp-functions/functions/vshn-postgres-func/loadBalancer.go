package vshnpostgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// AddLoadBalancerIPToConnectionDetails changes the desired state of a FunctionIO
func AddLoadBalancerIPToConnectionDetails(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}
	// Wait for the next reconciliation in case instance namespace is missing
	if comp.Status.InstanceNamespace == "" {
		return runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")
	}

	// if there is nothin to do, return early
	if comp.Spec.Parameters.Network.ServiceType != "LoadBalancer" {
		return runtime.NewNormal()
	}
	log.Info("Getting connection secret from managed kubernetes object")
	s := &v1.Secret{}

	err = iof.Observed.GetFromObject(ctx, s, connectionSecretResourceName)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get connection secret object", err)
	}

	log.Info("Setting ExternalLoadBalancerIP variable into connection secret")
	primary, replicas, err := getLoadBalancerIPs(comp.Status.InstanceNamespace)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get load balancer IPs", err)
	}
	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
		Name:  "PRIMARY_IP",
		Value: primary,
	})
	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
		Name:  "REPLICAS_IP",
		Value: replicas,
	})

	return runtime.NewNormal()
}

func getLoadBalancerIPs(instanceNamespace string) (primary string, replicas string, err error) {
	// Build the Kubernetes config
	// stackgres just creates services in the same namespace as the instance
	// therefore I need to manually check IP addresses and then return them
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", "", err
	}

	serviceName, _ := strings.CutPrefix(instanceNamespace, "vshn-postgresql-")

	// Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", err
	}

	// Get the list of services
	serviceList, err := clientset.CoreV1().Services(instanceNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	// Stackgres creates two services, one for the primary and one for the replicas
	// even if replicas is set to 1 we have 1 primary and 1 replicas service
	for _, svc := range serviceList.Items {
		if svc.Spec.Type == "LoadBalancer" {
			if svc.Name == serviceName {
				primary = svc.Status.LoadBalancer.Ingress[0].IP
			}
			if svc.Name == serviceName+"-replicas" {
				replicas = svc.Status.LoadBalancer.Ingress[0].IP
			}
		}
	}

	if primary == "" || replicas == "" {
		return "", "", fmt.Errorf("cannot find primary/replicas IP")
	}
	return primary, replicas, nil
}
