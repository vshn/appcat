package common

import (
	"context"
	"fmt"
	"testing"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/crossplane/function-sdk-go/proto/v1beta1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	v1 "k8s.io/api/core/v1"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	"github.com/stretchr/testify/assert"
)

func TestAddUserAlerting_NoInstanceNamespace(t *testing.T) {
	ctx := context.Background()

	t.Run("WhenNoInstance_ThenNoErrorAndNoChanges", func(t *testing.T) {

		//Given
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/05-GivenNoStatusInstanceNamespace.yaml")

		// Then
		runForGivenInputAlerting(t, ctx, svc, nil)
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
		expResult *xfnproto.Result
	}{
		{
			name: "GivenNoMonitoringParams_ThenExpectNoOutput",
			args: args{
				expectedFuncIO: "vshn-postgres/alerting/01-ThenExpectNoOutput.yaml",
				inputFuncIO:    "vshn-postgres/alerting/01-GivenNoMonitoringParams.yaml",
			},
			expResult: nil,
		},
		{
			name:      "GivenConfigRefNoSecretRef_ThenExpectError",
			expResult: runtime.NewFatalResult(fmt.Errorf("Found AlertmanagerConfigRef but no AlertmanagerConfigSecretRef, please specify as well")),
			args: args{
				expectedFuncIO: "vshn-postgres/alerting/02-ThenExpectError.yaml",
				inputFuncIO:    "vshn-postgres/alerting/02-GivenConfigRefNoSecretRef.yaml",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			svc := commontest.LoadRuntimeFromFile(t, tt.args.inputFuncIO)
			expSvc := commontest.LoadRuntimeFromFile(t, tt.args.expectedFuncIO)

			runForGivenInputAlerting(t, ctx, svc, nil)

			assert.Equal(t, expSvc, svc)
		})
	}
}

func TestGivenConfigRefAndSecretThenExpectOutput(t *testing.T) {

	ctx := context.Background()

	t.Run("GivenConfigRefAndSecret_ThenExpectOutput", func(t *testing.T) {

		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/03-GivenConfigRefAndSecret.yaml")

		runForGivenInputAlerting(t, ctx, svc, nil)

		resName := "psql-alertmanagerconfig"
		kubeObject := &xkube.Object{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, resName))

		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, svc.GetObservedComposite(comp))
		assert.Equal(t, comp.Labels["crossplane.io/claim-namespace"], kubeObject.Spec.References[0].PatchesFrom.Namespace)
		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigRef, kubeObject.Spec.References[0].PatchesFrom.Name)

		alertConfig := &alertmanagerv1alpha1.AlertmanagerConfig{}
		assert.NoError(t, svc.GetDesiredKubeObject(alertConfig, resName))
		assert.Equal(t, comp.Status.InstanceNamespace, alertConfig.GetNamespace())

		secretName := "psql-alertmanagerconfigsecret"
		secret := &v1.Secret{}
		assert.NoError(t, svc.GetDesiredKubeObject(secret, secretName))

		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSecretRef, secret.GetName())
	})
}

func TestGivenConfigTemplateAndSecretThenExpectOutput(t *testing.T) {
	ctx := context.Background()

	t.Run("GivenConfigTemplateAndSecret_ThenExpectOutput", func(t *testing.T) {

		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/alerting/04-GivenConfigTemplateAndSecret.yaml")

		runForGivenInputAlerting(t, ctx, svc, nil)

		resName := "psql-alertmanagerconfig"
		kubeObject := &xkube.Object{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, resName))

		assert.Empty(t, kubeObject.Spec.References)

		alertConfig := &alertmanagerv1alpha1.AlertmanagerConfig{}
		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, svc.GetDesiredKubeObject(alertConfig, resName))
		assert.NoError(t, svc.GetObservedComposite(comp))
		assert.Equal(t, comp.Status.InstanceNamespace, alertConfig.GetNamespace())
		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSpecTemplate, &alertConfig.Spec)

		secretName := "psql-alertmanagerconfigsecret"
		secret := &v1.Secret{}
		assert.NoError(t, svc.GetDesiredKubeObject(secret, secretName))

		assert.Equal(t, comp.Spec.Parameters.Monitoring.AlertmanagerConfigSecretRef, secret.GetName())
	})
}

func runForGivenInputAlerting(t *testing.T, ctx context.Context, input *runtime.ServiceRuntime, res *v1beta1.Result) {
	fnc := AddUserAlerting(&vshnv1.VSHNRedis{})

	assert.Equal(t, res, fnc(ctx, input))

	fnc = AddUserAlerting(&vshnv1.VSHNPostgreSQL{})

	assert.Equal(t, res, fnc(ctx, input))

}
