package common

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	runtime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MailgunAlerting[T client.Object](ctx context.Context, obj T, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting mailgun-alerting function")

	err := svc.GetObservedComposite(obj)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't get composite: %w", err))
	}
	objCopy := obj.DeepCopyObject()
	alertConfig, ok := objCopy.(Alerter)
	if !ok {
		return runtime.NewWarningResult(fmt.Sprintf("Type %s doesn't implement Alerter interface", reflect.TypeOf(obj).String()))
	}

	monitoring := alertConfig.GetVSHNMonitoring()
	email := monitoring.Email
	alertFrequency := monitoring.AlertFrequency
	instanceNamespace := alertConfig.GetInstanceNamespace()
	name := obj.GetName()

	if email == "" {
		return nil
	}
	if !mailAlertingEnabled(&svc.Config) {
		return runtime.NewWarningResult("Email Alerting is not enabled")
	}

	log.Info("Deploying AlertmanagerConfig for mail alerting...", "alertFrequency", alertFrequency)
	err = deployAlertmanagerConfig(ctx, name, email, alertFrequency, instanceNamespace, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy AlertmanagerConfig "+name+"-alertmanagerconfig-mailgun for mail alerting: %w", err))
	}

	log.Info("Finishing mailgun-alerting function with NewNormal")

	return nil
}

// getAlertIntervals returns the appropriate alert timing intervals.
// Priority: per-instance AlertFrequency preset > component ConfigMap values > hardcoded defaults.
//
// AlertFrequency presets:
//   - "frequent": 10s/5m/1h (legacy behavior, max ~24 emails/day)
//   - "standard": 30s/10m/4h (default, max ~6 emails/day)
//   - "minimal": 1m/30m/12h (low spam, max ~2 emails/day)
func getAlertIntervals(alertFrequency string, config *v1.ConfigMap) (groupWait, groupInterval, repeatInterval string) {
	// If per-instance AlertFrequency preset is specified, use it
	switch alertFrequency {
	case "frequent":
		return "10s", "5m", "1h"
	case "minimal":
		return "1m", "30m", "12h"
	case "standard":
		return "30s", "10m", "4h"
	}

	// Otherwise, use component ConfigMap values with hardcoded fallbacks
	groupWait = config.Data["emailAlertingGroupWait"]
	if groupWait == "" {
		groupWait = "30s"
	}

	groupInterval = config.Data["emailAlertingGroupInterval"]
	if groupInterval == "" {
		groupInterval = "10m"
	}

	repeatInterval = config.Data["emailAlertingRepeatInterval"]
	if repeatInterval == "" {
		repeatInterval = "4h"
	}

	return groupWait, groupInterval, repeatInterval
}

func deployAlertmanagerConfig(ctx context.Context, name, email, alertFrequency, instanceNamespace string, svc *runtime.ServiceRuntime) error {
	var alertManagerConfigName = name + "-alertmanagerconfig-mailgun"
	var alertManagerConfigSecretName = name + "-alertmanagerconfig-mailgun-secret"
	receiverName := "mailgun"

	groupWait, groupInterval, repeatInterval := getAlertIntervals(alertFrequency, &svc.Config)

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
				GroupWait:      groupWait,
				GroupInterval:  groupInterval,
				RepeatInterval: repeatInterval,
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
			Name: alertManagerConfigSecretName,
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

	if err := svc.SetDesiredKubeObject(secret, alertManagerConfigSecretName, runtime.KubeOptionAddRefs(patchSecretWithOtherSecret), runtime.KubeOptionAllowDeletion); err != nil {
		return err
	}

	return svc.SetDesiredKubeObject(ac, alertManagerConfigName, runtime.KubeOptionAddRefs(xRef), runtime.KubeOptionAllowDeletion)
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
