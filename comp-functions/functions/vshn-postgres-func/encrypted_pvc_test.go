package vshnpostgres

import (
	"context"
	"github.com/vshn/appcat-apiserver/comp-functions/runtime"
	"testing"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
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
		expResult xfnv1alpha1.Result
	}{
		{
			name: "GivenNoEncryptionParams_ThenExpectNoOutput",
			args: args{
				expectedFuncIO: "enc_pvc/01-ThenExpectNoOutput.yaml",
				inputFuncIO:    "enc_pvc/01-GivenNoEncryptionParams.yaml",
			},
			expResult: xfnv1alpha1.Result{
				Severity: xfnv1alpha1.SeverityNormal,
				Message:  "function ran successfully",
			},
		},
		{
			name: "GivenEncryptionParamsToFalse_ThenExpectFalseOutput",
			args: args{
				expectedFuncIO: "enc_pvc/02-ThenExpectFalseOutput.yaml",
				inputFuncIO:    "enc_pvc/02-GivenEncryptionParamsFalse.yaml",
			},
			expResult: xfnv1alpha1.Result{
				Severity: xfnv1alpha1.SeverityNormal,
				Message:  "function ran successfully",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			iof := loadRuntimeFromFile(t, tt.args.inputFuncIO)
			expIof := loadRuntimeFromFile(t, tt.args.expectedFuncIO)

			r := AddPvcSecret(ctx, iof)

			assert.Equal(t, tt.expResult, r.Resolve())
			assert.Equal(t, getFunctionIo(expIof), getFunctionIo(iof))
		})
	}
}

func TestGivenEncrypedPvcThenExpectOutput(t *testing.T) {

	ctx := context.Background()

	t.Run("GivenEncryptionEnabledNoInstanceNamespace_ThenExpectWarningOutput", func(t *testing.T) {

		iof := loadRuntimeFromFile(t, "enc_pvc/03-GivenEncryptionParamsNoInstanceNamespace.yaml")

		r := AddPvcSecret(ctx, iof)

		assert.Equal(t, runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation"), r)
	})

	t.Run("GivenEncryptionEnabled_ThenExpectOutput", func(t *testing.T) {

		iof := loadRuntimeFromFile(t, "enc_pvc/03-GivenEncryptionParams.yaml")

		r := AddPvcSecret(ctx, iof)

		assert.Equal(t, runtime.NewNormal(), r)

		comp := &vshnv1.VSHNPostgreSQL{}

		assert.NoError(t, iof.Observed.GetComposite(ctx, comp))

		resName := comp.Name + "-luks-key"
		kubeObject := &xkube.Object{}
		assert.NoError(t, iof.Desired.Get(ctx, kubeObject, resName))

		//resName := inComp.Name + "-luks-key"
		//assert.NoError(t, iof.Desired.GetManagedResource(resName, kubeObject))
		s := &v1.Secret{}
		assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
		assert.NotEmpty(t, s.Data["luksKey"])
	})

	t.Run("GivenEncryptionEnabledExistingSecret_ThenExpectOutput", func(t *testing.T) {

		iof := loadRuntimeFromFile(t, "enc_pvc/03-GivenEncryptionParamsExistingSecret.yaml")

		r := AddPvcSecret(ctx, iof)

		assert.Equal(t, runtime.NewNormal(), r)

		comp := &vshnv1.VSHNPostgreSQL{}

		assert.NoError(t, iof.Observed.GetComposite(ctx, comp))

		resName := comp.Name + "-luks-key"
		kubeObject := &xkube.Object{}
		assert.NoError(t, iof.Desired.Get(ctx, kubeObject, resName))

		//resName := inComp.Name + "-luks-key"
		//assert.NoError(t, iof.Desired.GetManagedResource(resName, kubeObject))
		s := &v1.Secret{}
		assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
		assert.NotEmpty(t, s.Data["luksKey"])
	})

}
