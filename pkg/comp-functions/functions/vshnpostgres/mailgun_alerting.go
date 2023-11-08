package vshnpostgres

import (
	"context"
	"fmt"
	"strconv"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	runtime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func MailgunAlerting(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting mailgun-alerting function")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	if !mailAlertingEnabled(&svc.Config) {
		if comp.Spec.Parameters.Monitoring.Email == "" {
			return nil
		}
		return runtime.NewWarningResult("Email Alerting is not enabled")
	}

	if comp.Spec.Parameters.Monitoring.Email == "" {
		return nil
	}

	log.Info("Deploying AlertmanagerConfig for mail alerting...")
	err = deployAlertmanagerConfig(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy AlertmanagerConfig "+comp.Name+"-alertmanagerconfig-mailgun for mail alerting: %w", err))
	}

	log.Info("Finishing mailgun-alerting function with NewNormal")

	return nil
}

func deployAlertmanagerConfig(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	var alertManagerConfigName = comp.Name + "-alertmanagerconfig-mailgun"
	var alertManagerConfigSecretName = comp.Name + "-alertmanagerconfig-mailgun-secret"
	receiverName := "mailgun"

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertManagerConfigName,
			Namespace: getInstanceNamespace(comp),
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
							To:           comp.Spec.Parameters.Monitoring.Email,
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
			Namespace: getInstanceNamespace(comp),
		},
	}

	xRef := xkube.Reference{
		DependsOn: &xkube.DependsOn{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  getInstanceNamespace(comp),
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
			FieldPath: pointer.String("data.password"),
		},
		ToFieldPath: pointer.String("data.password"),
	}

	if err := svc.SetDesiredKubeObject(secret, alertManagerConfigSecretName, patchSecretWithOtherSecret); err != nil {
		return err
	}

	return svc.SetDesiredKubeObject(ac, alertManagerConfigName, xRef)
}

func mailAlertingEnabled(config *corev1.ConfigMap) bool {
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
