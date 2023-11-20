package common

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	runtime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MailgunAlerting(obj client.Object) func(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	return func(ctx context.Context, iof *runtime.Runtime) runtime.Result {
		log := controllerruntime.LoggerFrom(ctx)
		log.Info("Starting mailgun-alerting function")

		err := iof.Observed.GetComposite(ctx, obj)
		if err != nil {
			return runtime.NewFatalErr(ctx, "Can't get composite", err)
		}
		alertConfig, ok := obj.(Alerter)
		if !ok {
			return runtime.NewWarning(ctx, fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
		}

		email := alertConfig.GetVSHNMonitoring().Email
		instanceNamespace := alertConfig.GetInstanceNamespace()
		name := obj.GetName()

		if email == "" {
			return runtime.NewNormal()
		}
		if !mailAlertingEnabled(iof.Config) {
			return runtime.NewWarning(ctx, "Email Alerting is not enabled")
		}

		log.Info("Deploying AlertmanagerConfig for mail alerting...")
		err = deployAlertmanagerConfig(ctx, name, email, instanceNamespace, iof)
		if err != nil {
			return runtime.NewFatalErr(ctx, "Can't deploy AlertmanagerConfig "+name+"-alertmanagerconfig-mailgun for mail alerting", err)
		}

		log.Info("Finishing mailgun-alerting function with NewNormal")

		return runtime.NewNormal()
	}
}

func deployAlertmanagerConfig(ctx context.Context, name, email, instanceNamespace string, iof *runtime.Runtime) error {
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
							From:         iof.Config.Data["emailAlertingSmtpFromAddress"],
							AuthUsername: iof.Config.Data["emailAlertingSmtpUsername"],
							AuthPassword: &v1.SecretKeySelector{
								Key: "password",
								LocalObjectReference: v1.LocalObjectReference{
									Name: alertManagerConfigSecretName,
								},
							},
							Smarthost:    iof.Config.Data["emailAlertingSmtpHost"],
							RequireTLS:   pointer.Bool(true),
							SendResolved: pointer.Bool(true),
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
				Namespace:  iof.Config.Data["emailAlertingSecretNamespace"],
				Name:       iof.Config.Data["emailAlertingSecretName"],
			},
			FieldPath: pointer.String("data.password"),
		},
		ToFieldPath: pointer.String("data.password"),
	}

	if err := iof.Desired.PutIntoObject(ctx, secret, alertManagerConfigSecretName, patchSecretWithOtherSecret); err != nil {
		return err
	}

	return iof.Desired.PutIntoObject(ctx, ac, alertManagerConfigName, xRef)
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
