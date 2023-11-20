package common

import (
	"context"
	"fmt"
	"reflect"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type Alerter interface {
	GetVSHNMonitoring() vshnv1.VSHNMonitoring
	GetInstanceNamespace() string
}

// AddUserAlerting adds user alerting to the Redis instance.
func AddUserAlerting(obj client.Object) func(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	return func(ctx context.Context, iof *runtime.Runtime) runtime.Result {

		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Check if alerting references are set")

		log.V(1).Info("Transforming", "obj", iof)

		err := iof.Observed.GetComposite(ctx, obj)
		if err != nil {
			return runtime.NewFatalErr(ctx, "Can't get composite", err)
		}
		alertConfig, ok := obj.(Alerter)
		if !ok {
			return runtime.NewWarning(ctx, fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
		}

		monitoringSpec := alertConfig.GetVSHNMonitoring()
		refName := monitoringSpec.AlertmanagerConfigRef

		if monitoringSpec.AlertmanagerConfigRef != "" {

			if monitoringSpec.AlertmanagerConfigSecretRef == "" {
				return runtime.NewFatal(ctx, "Found AlertmanagerConfigRef but no AlertmanagerConfigSecretRef, please specify as well")
			}

			log.Info("Found an AlertmanagerConfigRef, deploying...", "refName", refName)

			err = deployAlertmanagerFromRef(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), iof)
			if err != nil {
				return runtime.NewFatalErr(ctx, "Could not deploy alertmanager from ref", err)
			}
		}

		if monitoringSpec.AlertmanagerConfigSpecTemplate != nil {

			if monitoringSpec.AlertmanagerConfigSecretRef == "" {
				return runtime.NewFatal(ctx, "Found AlertmanagerConfigTemplate but no AlertmanagerConfigSecretRef, please specify as well")
			}

			log.Info("Found an AlertmanagerConfigTemplate, deploying...")

			err = deployAlertmanagerFromTemplate(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), monitoringSpec.AlertmanagerConfigSpecTemplate, iof)
			if err != nil {
				return runtime.NewFatalErr(ctx, "Cannot deploy alertmanager from template", err)
			}
		}

		if monitoringSpec.AlertmanagerConfigSecretRef != "" {
			refName := monitoringSpec.AlertmanagerConfigSecretRef
			log.Info("Found an AlertmanagerConfigSecretRef, deploying...", "refName", refName)

			err = deploySecretRef(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), iof)
			if err != nil {
				return runtime.NewFatalErr(ctx, "Cannot deploy secret ref", err)
			}
		}

		return runtime.NewNormal()
	}
}

func deployAlertmanagerFromRef(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, iof *runtime.Runtime) error {
	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-alertmanagerconfig",
			Namespace: instanceNamespace,
		},
	}

	xRef := xkube.Reference{
		PatchesFrom: &xkube.PatchesFrom{
			DependsOn: xkube.DependsOn{
				APIVersion: "monitoring.coreos.com/v1alpha1",
				Kind:       "AlertmanagerConfig",
				Namespace:  claimNamespace,
				Name:       AlertmanagerConfigSecretRef,
			},
			FieldPath: pointer.String("spec"),
		},
		ToFieldPath: pointer.String("spec"),
	}

	return iof.Desired.PutIntoObject(ctx, ac, name+"-alertmanagerconfig", xRef)
}

func deployAlertmanagerFromTemplate(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, AlertmanagerConfigSpecTemplate *alertmanagerv1alpha1.AlertmanagerConfigSpec, iof *runtime.Runtime) error {
	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AlertmanagerConfigSecretRef,
			Namespace: instanceNamespace,
		},
		Spec: *AlertmanagerConfigSpecTemplate,
	}

	return iof.Desired.PutIntoObject(ctx, ac, name+"-alertmanagerconfig")
}

func deploySecretRef(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, iof *runtime.Runtime) error {
	s := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AlertmanagerConfigSecretRef,
			Namespace: instanceNamespace,
		},
	}
	xRef := xkube.Reference{
		PatchesFrom: &xkube.PatchesFrom{
			DependsOn: xkube.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  claimNamespace,
				Name:       AlertmanagerConfigSecretRef,
			},
			FieldPath: pointer.String("data"),
		},
		ToFieldPath: pointer.String("data"),
	}

	return iof.Desired.PutIntoObject(ctx, s, name+"-alertmanagerconfigsecret", xRef)
}
