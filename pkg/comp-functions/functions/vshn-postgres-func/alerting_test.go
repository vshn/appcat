package vshnpostgres

import (
	"context"
	"github.com/vshn/appcat/pkg/comp-functions/functions/commontest"
	"testing"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestAddUserAlerting_NoInstanceNamespace(t *testing.T) {
	ctx := context.Background()
	expectResult := runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")

	t.Run("WhenNoInstance_ThenNoErrorAndNoChanges", func(t *testing.T) {

		//Given
		io := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/05-GivenNoStatusInstanceNamespace.yaml")

		// When
		result := AddUserAlerting(ctx, io)

		// Then
		assert.Equal(t, expectResult, result)
	})
}

func TestAddUserAlerting(t *testing.T) {
	ctx := context.Background()

	type args struct {
		expectedFuncIO string
		inputFuncIO    string
	}
	tests := []struct {
		name      string
		args      args
		expResult xfnv1alpha1.Result
	}{
		{
			name: "GivenNoMonitoringParams_ThenExpectNoOutput",
			args: args{
				expectedFuncIO: "vshn-postgres/alerting/01-ThenExpectNoOutput.yaml",
				inputFuncIO:    "vshn-postgres/alerting/01-GivenNoMonitoringParams.yaml",
			},
			expResult: xfnv1alpha1.Result{
				Severity: xfnv1alpha1.SeverityNormal,
				Message:  "function ran successfully",
			},
		},
		{
			name:      "GivenConfigRefNoSecretRef_ThenExpectError",
			expResult: runtime.NewFatal(ctx, "Found AlertmanagerConfigRef but no AlertmanagerConfigSecretRef, please specify as well").Resolve(),
			args: args{
				expectedFuncIO: "vshn-postgres/alerting/02-ThenExpectError.yaml",
				inputFuncIO:    "vshn-postgres/alerting/02-GivenConfigRefNoSecretRef.yaml",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			iof := commontest.LoadRuntimeFromFile(t, tt.args.inputFuncIO)
			expIof := commontest.LoadRuntimeFromFile(t, tt.args.expectedFuncIO)

			r := AddUserAlerting(ctx, iof)

			assert.Equal(t, tt.expResult, r.Resolve())
			assert.Equal(t, commontest.GetFunctionIo(expIof), commontest.GetFunctionIo(iof))
		})
	}
}

func TestGivenConfigRefAndSecretThenExpectOutput(t *testing.T) {

	ctx := context.Background()

	t.Run("GivenConfigRefAndSecret_ThenExpectOutput", func(t *testing.T) {

		iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/03-GivenConfigRefAndSecret.yaml")

		r := AddUserAlerting(ctx, iof)
		assert.Equal(t, runtime.NewNormal(), r)

		resName := "psql-alertmanagerconfig"
		kubeObject := &xkube.Object{}
		assert.NoError(t, iof.Desired.Get(ctx, kubeObject, resName))

		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, iof.Observed.GetComposite(ctx, comp))
		assert.Equal(t, comp.Labels["crossplane.io/claim-namespace"], kubeObject.Spec.References[0].PatchesFrom.Namespace)
		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigRef, kubeObject.Spec.References[0].PatchesFrom.Name)

		alertConfig := &alertmanagerv1alpha1.AlertmanagerConfig{}
		assert.NoError(t, iof.Desired.GetFromObject(ctx, alertConfig, resName))
		assert.Equal(t, comp.Status.InstanceNamespace, alertConfig.GetNamespace())

		secretName := "psql-alertmanagerconfigsecret"
		secret := &v1.Secret{}
		assert.NoError(t, iof.Desired.GetFromObject(ctx, secret, secretName))

		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSecretRef, secret.GetName())
	})
}

func TestGivenConfigTemplateAndSecretThenExpectOutput(t *testing.T) {
	ctx := context.Background()

	t.Run("GivenConfigTemplateAndSecret_ThenExpectOutput", func(t *testing.T) {

		iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/04-GivenConfigTemplateAndSecret.yaml")

		r := AddUserAlerting(ctx, iof)
		assert.Equal(t, runtime.NewNormal(), r)

		resName := "psql-alertmanagerconfig"
		kubeObject := &xkube.Object{}
		assert.NoError(t, iof.Desired.Get(ctx, kubeObject, resName))

		assert.Empty(t, kubeObject.Spec.References)

		alertConfig := &alertmanagerv1alpha1.AlertmanagerConfig{}
		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, iof.Desired.GetFromObject(ctx, alertConfig, resName))
		assert.NoError(t, iof.Observed.GetComposite(ctx, comp))
		assert.Equal(t, comp.Status.InstanceNamespace, alertConfig.GetNamespace())
		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSpecTemplate, &alertConfig.Spec)

		secretName := "psql-alertmanagerconfigsecret"
		secret := &v1.Secret{}
		assert.NoError(t, iof.Desired.GetFromObject(ctx, secret, secretName))

		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSecretRef, secret.GetName())
	})
}
