package common

// test cases for the function MailgunAlerting

import (
	"context"
	"encoding/json"
	"testing"

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

func runForGivenInputMailgun(t *testing.T, ctx context.Context, input *runtime.ServiceRuntime, res *xfnproto.Result) {
	fnc := MailgunAlerting[*vshnv1.VSHNRedis](context.TODO(), &vshnv1.VSHNRedis{}, input)

	assert.Equal(t, res, fnc)

	fnc = MailgunAlerting[*vshnv1.VSHNPostgreSQL](context.TODO(), &vshnv1.VSHNPostgreSQL{}, input)

	assert.Equal(t, res, fnc)

}
