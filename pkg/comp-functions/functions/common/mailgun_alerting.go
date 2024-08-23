package common

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	runtime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MailgunAlerting(obj client.Object, ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting mailgun-alerting function")

	err := svc.GetObservedComposite(obj)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't get composite: %w", err))
	}
	alertConfig, ok := obj.(Alerter)
	if !ok {
		return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
	}

	email := alertConfig.GetVSHNMonitoring().Email
	instanceNamespace := alertConfig.GetInstanceNamespace()
	name := obj.GetName()

	if email == "" {
		return nil
	}
	if !mailAlertingEnabled(&svc.Config) {
		return runtime.NewWarningResult("Email Alerting is not enabled")
	}

	log.Info("Deploying AlertmanagerConfig for mail alerting...")
	err = deployAlertmanagerConfig(ctx, name, email, instanceNamespace, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy AlertmanagerConfig "+name+"-alertmanagerconfig-mailgun for mail alerting: %w", err))
	}

	log.Info("Finishing mailgun-alerting function with NewNormal")

	return nil
}

func deployAlertmanagerConfig(ctx context.Context, name, email, instanceNamespace string, svc *runtime.ServiceRuntime) error {
	var alertManagerConfigName = name + "-alertmanagerconfig-mailgun"
	var alertManagerConfigSecretName = name + "-alertmanagerconfig-mailgun-secret"
	receiverName := "mailgun"

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertManagerConfigName,
			Namespace: instanceNamespace,
			Labels: map[string]string{
				"alert-manager-config": receiverName,
			},
		},
		Spec: alertmanagerv1alpha1.AlertmanagerConfigSpec{
			Receivers: []alertmanagerv1alpha1.Receiver{
				{
					Name: receiverName,
					EmailConfigs: []alertmanagerv1alpha1.EmailConfig{
						{
							To:           email,
							From:         svc.Config.Data["emailAlertingSmtpFromAddress"],
							AuthUsername: svc.Config.Data["emailAlertingSmtpUsername"],
							AuthPassword: &v1.SecretKeySelector{
								Key: "password",
								LocalObjectReference: v1.LocalObjectReference{
									Name: alertManagerConfigSecretName,
								},
							},
							Smarthost:    svc.Config.Data["emailAlertingSmtpHost"],
							RequireTLS:   ptr.To(true),
							SendResolved: ptr.To(true),
						},
					},
				},
			},
			Route: &alertmanagerv1alpha1.Route{
				GroupBy: []string{
					"alertname",
				},
				GroupWait:      "10s",
				GroupInterval:  "5m",
				RepeatInterval: "1h",
				Receiver:       receiverName,
			},
		},
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertManagerConfigSecretName,
			Namespace: instanceNamespace,
		},
	}

	xRef := xkube.Reference{
		DependsOn: &xkube.DependsOn{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  instanceNamespace,
			Name:       alertManagerConfigSecretName,
		},
	}

	patchSecretWithOtherSecret := xkube.Reference{
		PatchesFrom: &xkube.PatchesFrom{
			DependsOn: xkube.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  svc.Config.Data["emailAlertingSecretNamespace"],
				Name:       svc.Config.Data["emailAlertingSecretName"],
			},
			FieldPath: ptr.To("data.password"),
		},
		ToFieldPath: ptr.To("data.password"),
	}

	if err := svc.SetDesiredKubeObject(secret, alertManagerConfigSecretName, runtime.KubeOptionAddRefs(patchSecretWithOtherSecret)); err != nil {
		return err
	}

	return svc.SetDesiredKubeObject(ac, alertManagerConfigName, runtime.KubeOptionAddRefs(xRef))
}

func mailAlertingEnabled(config *v1.ConfigMap) bool {
	en, ok := config.Data["emailAlertingEnabled"]
	if !ok {
		return false
	}
	enabled, err := strconv.ParseBool(en)
	if err != nil {
		return false
	}
	return enabled
}
