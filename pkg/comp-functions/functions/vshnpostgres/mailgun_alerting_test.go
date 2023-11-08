package vshnpostgres

// test cases for the function MailgunAlerting

import (
	"context"
	"testing"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	// "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestMailgunAlerting(t *testing.T) {
	ctx := context.Background()

	// return Normal when there is no email configured
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/06-GivenNoEmail.yaml")

	result := MailgunAlerting(ctx, svc)

	assert.Nil(t, result)

	// return Normal and 2 new resources in Desired when email is provided
	svc = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/07-GivenEmail.yaml")

	result = MailgunAlerting(ctx, svc)
	assert.Nil(t, result)

	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	resNameMailgunSecret := "psql-alertmanagerconfig-mailgun-secret"
	kubeObjectMailgunSecret := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObjectMailgunSecret, resNameMailgunSecret))

	s := &v1.Secret{}

	assert.NoError(t, yaml.Unmarshal(kubeObjectMailgunSecret.Spec.ForProvider.Manifest.Raw, s))
	assert.Equal(t, resNameMailgunSecret, s.ObjectMeta.Name)

	resNameMailgun := "psql-alertmanagerconfig-mailgun"
	kubeObjectMailgun := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObjectMailgun, resNameMailgun))

	ac := &alertmanagerv1alpha1.AlertmanagerConfig{}
	assert.NoError(t, yaml.Unmarshal(kubeObjectMailgun.Spec.ForProvider.Manifest.Raw, ac))
	assert.Equal(t, resNameMailgun, ac.ObjectMeta.Name)

	// email is provided but empty, so return Normal and no new resources in Desired
	svc = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/08-GivenEmptyEmail.yaml")

	result = MailgunAlerting(ctx, svc)

	assert.Nil(t, result)
	desired := svc.GetAllDesired()
	assert.Empty(t, desired)

	svc = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/09-GivenEmailAlertingDisabled.yaml")

	result = MailgunAlerting(ctx, svc)

	desired = svc.GetAllDesired()
	assert.Empty(t, desired)

	svc = commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/10-GivenNoEmailAlertingDisabled.yaml")

	result = MailgunAlerting(ctx, svc)

	assert.Nil(t, result)
	desired = svc.GetAllDesired()
	assert.Empty(t, desired)
}
