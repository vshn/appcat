package vshnpostgres

import (
	"context"
	"testing"

	// xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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

		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/enc_pvc/03-GivenEncryptionParams.yaml")

		r := AddPvcSecret(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		assert.Equal(t, r, runtime.NewWarningResult("luks secret not yet ready"))

		comp := &vshnv1.VSHNPostgreSQL{}

		assert.NoError(t, svc.GetObservedComposite(comp))

		resName := comp.Name + "-luks-key-0"
		kubeObject := &xkube.Object{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, resName))

		s := &v1.Secret{}
		assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
		assert.NotEmpty(t, s.Data["luksKey"])
	})

	t.Run("GivenEncryptionEnabledExistingSecret_ThenExpectOutput", func(t *testing.T) {

		svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/enc_pvc/03-GivenEncryptionParamsExistingSecret.yaml")

		r := AddPvcSecret(ctx, &vshnv1.VSHNPostgreSQL{}, svc)

		assert.Nil(t, r)

		comp := &vshnv1.VSHNPostgreSQL{}

		assert.NoError(t, svc.GetObservedComposite(comp))

		resName := comp.Name + "-luks-key-0"
		kubeObject := &xkube.Object{}
		assert.NoError(t, svc.GetDesiredComposedResourceByName(kubeObject, resName))

		s := &v1.Secret{}
		assert.NoError(t, yaml.Unmarshal(kubeObject.Spec.ForProvider.Manifest.Raw, s))
		assert.NotEmpty(t, s.Data["luksKey"])

		cluster := &stackgresv1.SGCluster{}
		assert.NoError(t, svc.GetDesiredKubeObject(cluster, "cluster"))
		assert.Equal(t, pointer.String("ssd-encrypted"), cluster.Spec.Pods.PersistentVolume.StorageClass)
	})

}
