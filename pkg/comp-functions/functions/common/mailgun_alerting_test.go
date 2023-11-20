package common

// test cases for the function MailgunAlerting

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	v1 "k8s.io/api/core/v1"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	err := runtime.AddToScheme(alertmanagerv1alpha1.SchemeBuilder)
	if err != nil {
		panic(err)
	}
}

func TestMailgunAlerting(t *testing.T) {
	ctx := context.Background()

	// return Normal when there is no email configured
	inputFnio := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/06-GivenNoEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewNormal())

	// return Normal and 2 new resources in Desired when email is provided
	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/07-GivenEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewNormal())

	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, inputFnio.Observed.GetComposite(ctx, comp))

	resNameMailgunSecret := "psql-alertmanagerconfig-mailgun-secret"
	kubeObjectMailgunSecret := &xkube.Object{}
	assert.NoError(t, inputFnio.Desired.Get(ctx, kubeObjectMailgunSecret, resNameMailgunSecret))

	s := &v1.Secret{}

	assert.NoError(t, json.Unmarshal(kubeObjectMailgunSecret.Spec.ForProvider.Manifest.Raw, s))
	assert.Equal(t, resNameMailgunSecret, s.ObjectMeta.Name)

	resNameMailgun := "psql-alertmanagerconfig-mailgun"
	kubeObjectMailgun := &xkube.Object{}
	assert.NoError(t, inputFnio.Desired.Get(ctx, kubeObjectMailgun, resNameMailgun))

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{}
	assert.NoError(t, json.Unmarshal(kubeObjectMailgun.Spec.ForProvider.Manifest.Raw, ac))
	assert.Equal(t, resNameMailgun, ac.ObjectMeta.Name)

	// email is provided but empty, so return Normal and no new resources in Desired
	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/08-GivenEmptyEmail.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewNormal())

	assert.Empty(t, inputFnio.Desired.List(ctx))

	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/09-GivenEmailAlertingDisabled.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewWarning(context.Background(), "Email Alerting is not enabled"))

	assert.Empty(t, inputFnio.Desired.List(ctx))

	inputFnio = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/10-GivenNoEmailAlertingDisabled.yaml")

	runForGivenInputMailgun(t, ctx, inputFnio, runtime.NewNormal())

	assert.Empty(t, inputFnio.Desired.List(ctx))
}

func runForGivenInputMailgun(t *testing.T, ctx context.Context, input *runtime.Runtime, res runtime.Result) {
	fnc := MailgunAlerting(&vshnv1.VSHNRedis{})

	assert.Equal(t, res, fnc(ctx, input))

	fnc = MailgunAlerting(&vshnv1.VSHNPostgreSQL{})

	assert.Equal(t, res, fnc(ctx, input))

}
