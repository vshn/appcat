package vshnpostgres

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	stackgresv1 "github.com/vshn/appcat/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var serviceObserverName = "loadbalancer-observer"

// // AddLoadBalancerIPToConnectionDetails changes the desired state of a FunctionIO
// func AddLoadBalancerIPToConnectionDetails(ctx context.Context, iof *runtime.Runtime) runtime.Result {
// 	log := controllerruntime.LoggerFrom(ctx)

// 	comp := &vshnv1.VSHNPostgreSQL{}
// 	err := iof.Desired.GetComposite(ctx, comp)
// 	if err != nil {
// 		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
// 	}

// 	cluster := &stackgresv1.SGCluster{}

// 	serviceToApply := &v1.Service{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "Service",
// 			APIVersion: "v1",
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      comp.GetName(),
// 			Namespace: getInstanceNamespace(comp),
// 			Annotations: map[string]string{
// 				"appcat.io/observe-only": "true",
// 			},
// 		},
// 	}

// 	// if there is nothin to do, return early
// 	if comp.Spec.Parameters.Network.ServiceType != "LoadBalancer" {
// 		return runtime.NewNormal()
// 	} else {
// 		err = iof.Desired.GetFromObject(ctx, cluster, "cluster")
// 		if err != nil {
// 			return runtime.NewFatalErr(ctx, "Cannot get cluster from function io", err)
// 		}
// 		cluster.Spec.PostgresServices = &stackgresv1.SGClusterSpecPostgresServices{
// 			Primary: &stackgresv1.SGClusterSpecPostgresServicesPrimary{
// 				Type: pointer.String("LoadBalancer"),
// 			},
// 		}
// 		err = iof.Desired.PutIntoObject(ctx, cluster, "cluster")
// 		if err != nil {
// 			return runtime.NewFatalErr(ctx, "Cannot put cluster into function io", err)
// 		}
// 	}
// 	// I must create it otherwise crossplane would remove that
// 	err = iof.Desired.PutIntoObject(ctx, serviceToApply, fmt.Sprintf("%s-%s", comp.GetName(), serviceObserverName))
// 	if err != nil {
// 		return runtime.NewFatalErr(ctx, "Cannot add loadbalancer observer", err)
// 	}

// 	// get the service observer object
// 	err = iof.Observed.GetFromObject(ctx, serviceToApply, fmt.Sprintf("%s-%s", comp.GetName(), serviceObserverName))
// 	if err != nil {
// 		return runtime.NewWarning(ctx, "Cannot yet get service observer object")
// 	}

// 	if serviceToApply.Status.LoadBalancer.Ingress == nil {
// 		return runtime.NewWarning(ctx, "LoadBalancerIP is not ready yet")
// 	}

// 	log.Info("Getting connection secret from managed kubernetes object")
// 	s := &v1.Secret{}

// 	err = iof.Observed.GetFromObject(ctx, s, connectionSecretResourceName)
// 	if err != nil {
// 		return runtime.NewFatalErr(ctx, "Cannot get connection secret object", err)
// 	}

// 	log.Info("Setting ExternalLoadBalancerIP variable into connection secret")
// 	iof.Desired.PutCompositeConnectionDetail(ctx, v1alpha1.ExplicitConnectionDetail{
// 		Name:  "LOADBALANCER_IP",
// 		Value: serviceToApply.Status.LoadBalancer.Ingress[0].IP,
// 	})

// 	return runtime.NewNormal()
// }

func AddLoadBalancerIPToConnectionDetails(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	//log := controllerruntime.LoggerFrom(ctx)

	comp, err := getVSHNPostgreSQL(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}

	if comp.Spec.Parameters.Network.ServiceType != "LoadBalancer" {
		return runtime.NewNormal()
	}

	cluster, err := getSGCluster(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get cluster from function io", err)
	}
	if err := updateClusterForLoadBalancer(cluster, ctx, iof); err != nil {
		return runtime.NewFatal(ctx, "Cannot put cluster into function io")
	}

	serviceToApply, err := createServiceForLoadBalancer(comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot create load balancer service", err)
	}

	if err := addServiceObserverToDesiredState(ctx, iof, serviceToApply); err != nil {
		return runtime.NewFatalErr(ctx, "Cannot add load balancer observer", err)
	}

	serviceToApply, err = getObservedService(ctx, iof, serviceToApply)
	if err != nil {
		return runtime.NewWarning(ctx, "Cannot yet get service observer object")
	}

	if serviceToApply.Status.LoadBalancer.Ingress == nil {
		return runtime.NewWarning(ctx, "LoadBalancerIP is not ready yet")
	}

	if err := updateConnectionSecretWithLoadBalancerIP(ctx, iof, serviceToApply); err != nil {
		return runtime.NewFatalErr(ctx, "Cannot update connection secret", err)
	}

	return runtime.NewNormal()
}

func getVSHNPostgreSQL(ctx context.Context, iof *runtime.Runtime) (*vshnv1.VSHNPostgreSQL, error) {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, comp)
	return comp, err
}

func getSGCluster(ctx context.Context, iof *runtime.Runtime) (*stackgresv1.SGCluster, error) {
	cluster := &stackgresv1.SGCluster{}
	err := iof.Desired.GetFromObject(ctx, cluster, "cluster")
	return cluster, err
}

func updateClusterForLoadBalancer(cluster *stackgresv1.SGCluster, ctx context.Context, iof *runtime.Runtime) error {
	cluster.Spec.PostgresServices = &stackgresv1.SGClusterSpecPostgresServices{
		Primary: &stackgresv1.SGClusterSpecPostgresServicesPrimary{
			Type: pointer.String("LoadBalancer"),
		},
	}
	err := iof.Desired.PutIntoObject(ctx, cluster, "cluster")
	if err != nil {
		return err
	}
	return nil
}

func createServiceForLoadBalancer(comp *vshnv1.VSHNPostgreSQL) (*v1.Service, error) {
	serviceToApply := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: getInstanceNamespace(comp),
		},
	}

	return serviceToApply, nil
}

func addServiceObserverToDesiredState(ctx context.Context, iof *runtime.Runtime, service *v1.Service) error {
	return iof.Desired.PutIntoObserveOnlyObject(ctx, service, fmt.Sprintf("%s-%s", service.Name, serviceObserverName))
}

func getObservedService(ctx context.Context, iof *runtime.Runtime, service *v1.Service) (*v1.Service, error) {
	err := iof.Observed.GetFromObject(ctx, service, fmt.Sprintf("%s-%s", service.Name, serviceObserverName))
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
