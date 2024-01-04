package common

import (
	"context"
	"fmt"
	"reflect"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	fnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
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
func AddUserAlerting(obj client.Object) func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {
	return func(ctx context.Context, svc *runtime.ServiceRuntime) *fnproto.Result {

		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Check if alerting references are set")

		log.V(1).Info("Transforming", "obj", svc)

		err := svc.GetObservedComposite(obj)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("Can't get composite: %w", err))
		}
		alertConfig, ok := obj.(Alerter)
		if !ok {
			return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
		}

		monitoringSpec := alertConfig.GetVSHNMonitoring()
		refName := monitoringSpec.AlertmanagerConfigRef

		if monitoringSpec.AlertmanagerConfigRef != "" {

			if monitoringSpec.AlertmanagerConfigSecretRef == "" {
				return runtime.NewFatalResult(fmt.Errorf("Found AlertmanagerConfigRef but no AlertmanagerConfigSecretRef, please specify as well"))
			}

			log.Info("Found an AlertmanagerConfigRef, deploying...", "refName", refName)

			err = deployAlertmanagerFromRef(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), svc)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("Could not deploy alertmanager from ref: %w", err))
			}
		}

		if monitoringSpec.AlertmanagerConfigSpecTemplate != nil {

			if monitoringSpec.AlertmanagerConfigSecretRef == "" {
				return runtime.NewFatalResult(fmt.Errorf("Found AlertmanagerConfigTemplate but no AlertmanagerConfigSecretRef, please specify as well"))
			}

			log.Info("Found an AlertmanagerConfigTemplate, deploying...")

			err = deployAlertmanagerFromTemplate(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), monitoringSpec.AlertmanagerConfigSpecTemplate, svc)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("Cannot deploy alertmanager from template: %w", err))
			}
		}

		if monitoringSpec.AlertmanagerConfigSecretRef != "" {
			refName := monitoringSpec.AlertmanagerConfigSecretRef
			log.Info("Found an AlertmanagerConfigSecretRef, deploying...", "refName", refName)

			err = deploySecretRef(ctx, refName, obj.GetLabels()["crossplane.io/claim-namespace"], obj.GetName(), alertConfig.GetInstanceNamespace(), svc)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("Cannot deploy secret ref: %w", err))
			}
		}

		return nil
	}
}

func deployAlertmanagerFromRef(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, svc *runtime.ServiceRuntime) error {
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

	return svc.SetDesiredKubeObject(ac, name+"-alertmanagerconfig", xRef)
}

func deployAlertmanagerFromTemplate(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, AlertmanagerConfigSpecTemplate *alertmanagerv1alpha1.AlertmanagerConfigSpec, svc *runtime.ServiceRuntime) error {
	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AlertmanagerConfigSecretRef,
			Namespace: instanceNamespace,
		},
		Spec: *AlertmanagerConfigSpecTemplate,
	}

	return svc.SetDesiredKubeObject(ac, name+"-alertmanagerconfig")
}

func deploySecretRef(ctx context.Context, AlertmanagerConfigSecretRef, claimNamespace, name, instanceNamespace string, svc *runtime.ServiceRuntime) error {
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

	return svc.SetDesiredKubeObject(s, name+"-alertmanagerconfigsecret", xRef)
}