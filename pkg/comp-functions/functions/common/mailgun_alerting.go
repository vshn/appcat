package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"text/template"
	"text/template/parse"

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

	email := alertConfig.GetVSHNMonitoring().Email
	instanceNamespace := alertConfig.GetInstanceNamespace()
	name := obj.GetName()

	if email == "" {
		return nil
	}

	if !mailAlertingEnabled(&svc.Config) {
		return runtime.NewWarningResult("Email Alerting is not enabled")
	}

	tmplData := emailTemplateData{
		Name:              name,
		InstanceNamespace: instanceNamespace,
		ClaimName:         obj.GetLabels()["crossplane.io/claim-name"],
		ClaimNamespace:    obj.GetLabels()["crossplane.io/claim-namespace"],
		Annotations:       obj.GetAnnotations(),
	}

	mail, renderErr := renderEmail(svc.Config.Data, tmplData)

	// Fallback to default if template is not successfully rendered
	var warning *xfnproto.Result
	if renderErr != nil {
		mail = renderedEmail{}
		warning = runtime.NewWarningResult(fmt.Sprintf("Can't render email templates, Alertmanager defaults used: %s", renderErr))
	}

	log.Info("Deploying AlertmanagerConfig for mail alerting...")
	err = deployAlertmanagerConfig(ctx, name, email, instanceNamespace, mail, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy AlertmanagerConfig "+name+"-alertmanagerconfig-mailgun for mail alerting: %w", err))
	}

	log.Info("Finishing mailgun-alerting function with NewNormal")

	return warning
}

// emailTemplateData is the [[ ]] render context. Annotations come from the
// claim via the composite and are reachable only through the annotation func.
type emailTemplateData struct {
	Name              string
	InstanceNamespace string
	ClaimName         string
	ClaimNamespace    string
	Annotations       map[string]string
}

// renderedEmail pairs the two, so they can't be swapped at a call site.
type renderedEmail struct {
	body    string
	subject string
}

// renderEmail resolves both templates, reporting either failure.
func renderEmail(config map[string]string, data emailTemplateData) (renderedEmail, error) {
	body, bodyErr := renderEmailTemplate(config["emailAlertingTemplateHTML"], data)
	if bodyErr != nil {
		bodyErr = fmt.Errorf("html template: %w", bodyErr)
	}
	subject, subjectErr := renderEmailTemplate(config["emailAlertingTemplateSubject"], data)
	if subjectErr != nil {
		subjectErr = fmt.Errorf("subject template: %w", subjectErr)
	}
	return renderedEmail{body: body, subject: subject}, errors.Join(bodyErr, subjectErr)
}

// renderEmailTemplate resolves [[ ]], leaving {{ }} for Alertmanager. Values are
// quoted so Alertmanager binds them as data, not as template source.
func renderEmailTemplate(tpl string, data emailTemplateData) (string, error) {
	if tpl == "" {
		return "", nil
	}

	funcs := template.FuncMap{
		"annotation": func(key string) string { return strconv.Quote(data.Annotations[key]) },
	}
	t, err := template.New("email").Delims("[[", "]]").Funcs(funcs).Parse(tpl)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, quoteValues(data)); err != nil {
		return "", err
	}

	// Alertmanager parses this again. Check if valid
	rendered := buf.String()
	tree := parse.New("alertmanager")
	tree.Mode = parse.SkipFuncCheck
	if _, err := tree.Parse(rendered, "", "", map[string]*parse.Tree{}); err != nil {
		return "", fmt.Errorf("invalid for Alertmanager: %w", err)
	}

	return rendered, nil
}

func quoteValues(d emailTemplateData) emailTemplateData {
	return emailTemplateData{
		Name:              strconv.Quote(d.Name),
		InstanceNamespace: strconv.Quote(d.InstanceNamespace),
		ClaimName:         strconv.Quote(d.ClaimName),
		ClaimNamespace:    strconv.Quote(d.ClaimNamespace),
	}
}

func deployAlertmanagerConfig(ctx context.Context, name, email, instanceNamespace string, mail renderedEmail, svc *runtime.ServiceRuntime) error {
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
							HTML:         mail.body,
							Headers:      subjectHeader(mail.subject),
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

func subjectHeader(subject string) []alertmanagerv1alpha1.KeyValue {
	if subject == "" {
		return nil
	}
	return []alertmanagerv1alpha1.KeyValue{{Key: "Subject", Value: subject}}
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
