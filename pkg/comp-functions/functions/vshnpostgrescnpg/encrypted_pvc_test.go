package vshnpostgrescnpg

import (
	"context"
	"testing"

	// xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestNoEncryptedPVC(t *testing.T) {
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
			name: "GivenNoEncryptionParams_ThenExpectNoOutput",
			args: args{
				expectedFuncIO: "vshn-postgres/enc_pvc/01-ThenExpectNoOutput.yaml",
				inputFuncIO:    "vshn-postgres/enc_pvc/01-GivenNoEncryptionParams.yaml",
			},
			expResult: nil,
		},
		{
			name: "GivenEncryptionParamsToFalse_ThenExpectFalseOutput",
			args: args{
				expectedFuncIO: "vshn-postgres/enc_pvc/02-ThenExpectFalseOutput.yaml",
				inputFuncIO:    "vshn-postgres/enc_pvc/02-GivenEncryptionParamsFalse.yaml",
			},
			expResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			svc := commontest.LoadRuntimeFromFile(t, tt.args.inputFuncIO)
			expSvc := commontest.LoadRuntimeFromFile(t, tt.args.expectedFuncIO)

			r := AddPvcSecret(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

			assert.Equal(t, tt.expResult, r)
			assert.Equal(t, expSvc, svc)
		})
	}
}

func TestGivenEncrypedPvcThenExpectOutput(t *testing.T) {
	ctx := context.Background()

	t.Run("GivenEncryptionEnabled_ThenExpectOutput", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/enc_pvc/03-GivenEncryptionParamsCnpg.yaml")

		r := AddPvcSecret(ctx, &vshnv1.VSHNPostgreSQL{}, svc)
		assert.Equal(t, runtime.NewWarningResult("luks secret 'psql-cluster-1-luks-key' not yet ready"), r)

		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, svc.GetObservedComposite(comp))

		t.Logf("comp get name: %s", comp.GetName())

		instances, err := getClusterInstancesReportedByCd(svc, comp)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(instances))

		luksKeyNames := getLuksKeyNames(instances)
		assert.Equal(t, 4, len(luksKeyNames))
		// 2 * amount of instances, as each instance gets 2 PVCs (data and WAL)

		for _, v := range getLuksKeyNames(instances) {
			t.Logf("Checking secret '%s'...", v)
			kubeObject := &xkube.Object{}
			assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, v))

			s := &v1.Secret{}
			assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
			assert.NotEmpty(t, s.Data["luksKey"])
		}
	})

	t.Run("GivenEncryptionEnabledExistingSecret_ThenExpectOutput", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/enc_pvc/03-GivenEncryptionParamsExistingSecretCnpg.yaml")

		cnpgcomp := &vshnv1.VSHNPostgreSQL{
			Spec: vshnv1.VSHNPostgreSQLSpec{
				Parameters: vshnv1.VSHNPostgreSQLParameters{
					Instances: 3,
				},
			},
		}

		r := AddPvcSecret(ctx, cnpgcomp, svc)
		assert.Nil(t, r)

		comp := &vshnv1.VSHNPostgreSQL{}
		assert.NoError(t, svc.GetObservedComposite(comp))

		instances, err := getClusterInstancesReportedByCd(svc, comp)
		assert.NoError(t, err)

		t.Logf("Got the following cluster instances: %v", instances)

		for _, v := range getLuksKeyNames(instances) {
			t.Logf("Checking secret '%s'...", v)
			kubeObject := &xkube.Object{}
			assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, v))

			s := &v1.Secret{}
			assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
			assert.NotEmpty(t, s.Data["luksKey"])
		}

		// Get values
		values, err := createCnpgHelmValues(ctx, svc, comp)
		assert.NoError(t, err)
		assert.NotNil(t, values)
		assert.Equal(t, "ssd-encrypted", values["cluster"].(map[string]any)["storage"].(map[string]any)["storageClass"])
	})
}
