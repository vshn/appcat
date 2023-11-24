package vshnminio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	minioproviderv1 "github.com/vshn/provider-minio/apis/provider/v1"
)

func TestDeployMinioProviderConfig(t *testing.T) {
	svc, comp := getMinioComp(t)

	ctx := context.TODO()

	assert.Nil(t, DeployMinioProviderConfig(ctx, svc))

	config := &minioproviderv1.ProviderConfig{}

	assert.NoError(t, svc.GetDesiredKubeObject(config, comp.GetName()+"-providerconfig"))

}
