package vshnminio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	minioproviderv1 "github.com/vshn/provider-minio/apis/provider/v1"
)

func TestDeployMinioProviderConfig(t *testing.T) {
	iof, comp := getMinioComp(t)

	ctx := context.TODO()

	assert.NoError(t, iof.Desired.SetComposite(ctx, comp))

	assert.Equal(t, runtime.NewNormal(), DeployMinioProviderConfig(ctx, iof))

	config := &minioproviderv1.ProviderConfig{}

	assert.NoError(t, iof.Desired.GetFromObject(ctx, config, comp.GetName()+"-providerconfig"))

}
