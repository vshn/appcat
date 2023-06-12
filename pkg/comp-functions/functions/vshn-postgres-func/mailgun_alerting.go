package vshnpostgres

import (
	"context"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	runtime "github.com/vshn/appcat/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func MailgunAlerting(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting mailgun-alerting function")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}
	// Wait for the next reconciliation in case instance namespace is missing
	if comp.Status.InstanceNamespace == "" {
		return runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")
	}

	if comp.Spec.Parameters.Monitoring.Email == "" {
		return runtime.NewNormal()
	}

	log.Info("Deploying AlertmanagerConfig for mail alerting...")
	err = deployAlertmanagerConfig(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Can't deploy AlertmanagerConfig "+comp.Name+"-alertmanagerconfig-mailgun for mail alerting", err)
	}

	log.Info("Finishing mailgun-alerting function with NewNormal")

	return runtime.NewNormal()
}

func deployAlertmanagerConfig(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, iof *runtime.Runtime) error {
	var alertManagerConfigName = comp.Name + "-alertmanagerconfig-mailgun"
	var alertManagerConfigSecretName = comp.Name + "-alertmanagerconfig-mailgun-secret"
	receiverName := "mailgun"

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertManagerConfigName,
			Namespace: comp.Status.InstanceNamespace,
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
			Namespace: comp.Status.InstanceNamespace,
		},
	}

	xRef := xkube.Reference{
		DependsOn: &xkube.DependsOn{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  comp.Status.InstanceNamespace,
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
