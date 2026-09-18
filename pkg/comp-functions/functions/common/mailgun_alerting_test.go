package common

// test cases for the function MailgunAlerting

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"text/template"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	v1 "k8s.io/api/core/v1"

	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func TestMailgunAlerting(t *testing.T) {
	ctx := context.Background()

	// return Normal when there is no email configured
	inputFnio := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/06-GivenNoEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, nil)

	// return Normal and 2 new resources in Desired when email is provided
	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/07-GivenEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, nil)

	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, inputFnio.GetObservedComposite(comp))

	resNameMailgunSecret := "psql-alertmanagerconfig-mailgun-secret"
	kubeObjectMailgunSecret := &xkube.Object{}
	assert.NoError(t, inputFnio.GetDesiredComposedResourceByName(kubeObjectMailgunSecret, resNameMailgunSecret))

	s := &v1.Secret{}

	assert.NoError(t, json.Unmarshal(kubeObjectMailgunSecret.Spec.ForProvider.Manifest.Raw, s))
	assert.Equal(t, resNameMailgunSecret, s.ObjectMeta.Name)

	resNameMailgun := "psql-alertmanagerconfig-mailgun"
	kubeObjectMailgun := &xkube.Object{}
	assert.NoError(t, inputFnio.GetDesiredComposedResourceByName(kubeObjectMailgun, resNameMailgun))

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{}
	assert.NoError(t, json.Unmarshal(kubeObjectMailgun.Spec.ForProvider.Manifest.Raw, ac))
	assert.Equal(t, resNameMailgun, ac.ObjectMeta.Name)

	// no templates configured, so the fields must be absent, not empty
	manifest := string(kubeObjectMailgun.Spec.ForProvider.Manifest.Raw)
	assert.NotContains(t, manifest, `"html"`)
	assert.NotContains(t, manifest, `"headers"`)

	// email is provided but empty, so return Normal and no new resources in Desired
	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/08-GivenEmptyEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, nil)

	assert.Empty(t, inputFnio.GetAllDesired())

	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/09-GivenEmailAlertingDisabled.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewWarningResult("Email Alerting is not enabled"))

	assert.Empty(t, inputFnio.GetAllDesired())

	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/10-GivenNoEmailAlertingDisabled.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, nil)

	assert.Empty(t, inputFnio.GetAllDesired())
}

func TestRenderEmailTemplate(t *testing.T) {
	data := emailTemplateData{
		Name:              "psql",
		InstanceNamespace: "vshn-postgresql-psql",
		ClaimName:         "psql",
		ClaimNamespace:    "org-acme",
		Annotations: map[string]string{
			"example.com/instance-url": "https://portal.example.com/instances/psql-42/",
		},
	}

	// no template, Alertmanager keeps its own body
	body, err := renderEmailTemplate("", data)
	assert.NoError(t, err)
	assert.Empty(t, body)

	// [[ ]] becomes quoted literals, {{ }} passes through
	body, err = renderEmailTemplate(
		`{{ $url := [[ annotation "example.com/instance-url" ]] }}{{ $name := [[ .ClaimName ]] }}`+
			`{{ range .Alerts }}{{ .Labels.alertname }}{{ end }}<a href="{{ $url }}">{{ $name }}</a>`,
		data,
	)
	assert.NoError(t, err)
	assert.Equal(t,
		`{{ $url := "https://portal.example.com/instances/psql-42/" }}{{ $name := "psql" }}`+
			`{{ range .Alerts }}{{ .Labels.alertname }}{{ end }}<a href="{{ $url }}">{{ $name }}</a>`,
		body)

	// missing annotation binds empty, so pass 2 can guard on it
	body, err = renderEmailTemplate(`{{ $url := [[ annotation "example.com/instance-url" ]] }}`, emailTemplateData{})
	assert.NoError(t, err)
	assert.Equal(t, `{{ $url := "" }}`, body)

	// quotes inside an attribute must survive, why pass 1 is not html/template
	body, err = renderEmailTemplate(
		`<td bgcolor="{{ if eq .CommonLabels.severity "critical" }}#B93725{{ else }}#6B3FB5{{ end }}">`,
		data,
	)
	assert.NoError(t, err)
	assert.Equal(t, `<td bgcolor="{{ if eq .CommonLabels.severity "critical" }}#B93725{{ else }}#6B3FB5{{ end }}">`, body)

	// annotations are user input, they must never become template source
	for name, payload := range map[string]string{
		"action":   `{{ .ExternalURL }}`,
		"printf":   "{{ printf `%cimg src=x onerror=alert(1)%c` 60 62 | safeHtml }}",
		"quotes":   `" onclick="steal()"><script>evil()</script>`,
		"newlines": "a\nb",
		"breakout": `foo" }}{{ .Secret }}{{ $x := "`,
	} {
		body, err := renderEmailTemplate(
			`{{ $v := [[ annotation "k" ]] }}`,
			emailTemplateData{Annotations: map[string]string{"k": payload}},
		)
		assert.NoError(t, err, name)
		assert.Equal(t, `{{ $v := `+strconv.Quote(payload)+` }}`, body, name)

		// Alertmanager prints it back unchanged
		tmpl, err := template.New("").Parse(body + `{{ $v }}`)
		assert.NoError(t, err, name)
		var out bytes.Buffer
		assert.NoError(t, tmpl.Execute(&out, nil), name)
		assert.Equal(t, payload, out.String(), name)
	}

	// broken [[ ]] errors
	_, err = renderEmailTemplate("[[ .Unclosed ", data)
	assert.Error(t, err)

	// broken {{ }} too, it would otherwise drop the mail at send time
	_, err = renderEmailTemplate("{{ .Unclosed ", data)
	assert.Error(t, err)

	// Alertmanager functions are accepted
	_, err = renderEmailTemplate(`{{ .Status | toUpper }} {{ .CommonAnnotations.summary | safeHtml }}`, data)
	assert.NoError(t, err)

}

func TestMailgunAlertingWithTemplate(t *testing.T) {
	inputFnio := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/11-GivenEmailTemplate.yaml")
	assert.Nil(t, MailgunAlerting[*vshnv1.VSHNPostgreSQL](context.TODO(), &vshnv1.VSHNPostgreSQL{}, inputFnio))

	kubeObject := &xkube.Object{}
	assert.NoError(t, inputFnio.GetDesiredComposedResourceByName(kubeObject, "psql-alertmanagerconfig-mailgun"))
	ac := &alertmanagerv1alpha1.AlertmanagerConfig{}
	assert.NoError(t, json.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, ac))

	ec := ac.Spec.Receivers[0].EmailConfigs[0]
	assert.Equal(t, `{{ $url := "https://portal.example.com/instances/pgsql-7/" }}<a href="{{ $url }}">{{ $url }}</a>`, ec.HTML)
	assert.Equal(t, []alertmanagerv1alpha1.KeyValue{{Key: "Subject", Value: `[{{ .Status }}] {{ $n := "pgsql" }}{{ $n }}`}}, ec.Headers)
}

func TestMailgunAlertingBrokenTemplate(t *testing.T) {
	inputFnio := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/12-GivenBrokenEmailTemplate.yaml")
	res := MailgunAlerting[*vshnv1.VSHNPostgreSQL](context.TODO(), &vshnv1.VSHNPostgreSQL{}, inputFnio)

	// warning, never fatal
	assert.NotNil(t, res)
	assert.Equal(t, xfnproto.Severity_SEVERITY_WARNING, res.Severity)
	assert.Contains(t, res.Message, "html template")

	// both fields drop, a custom body under the default subject leaks syn_team
	kubeObject := &xkube.Object{}
	assert.NoError(t, inputFnio.GetDesiredComposedResourceByName(kubeObject, "psql-alertmanagerconfig-mailgun"))
	ac := &alertmanagerv1alpha1.AlertmanagerConfig{}
	assert.NoError(t, json.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, ac))
	assert.Empty(t, ac.Spec.Receivers[0].EmailConfigs[0].HTML)
	assert.Empty(t, ac.Spec.Receivers[0].EmailConfigs[0].Headers)
}

func runForGivenInputMailgun(t *testing.T, ctx context.Context, input *runtime.ServiceRuntime, res *xfnproto.Result) {
	fnc := MailgunAlerting[*vshnv1.VSHNRedis](context.TODO(), &vshnv1.VSHNRedis{}, input)

	assert.Equal(t, res, fnc)

	fnc = MailgunAlerting[*vshnv1.VSHNPostgreSQL](context.TODO(), &vshnv1.VSHNPostgreSQL{}, input)

	assert.Equal(t, res, fnc)

}
