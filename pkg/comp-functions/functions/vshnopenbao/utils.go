package vshnopenbao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func getOpenBaoTestComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNOpenBao) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/deploy/01_default.yaml")

	comp := &vshnv1.VSHNOpenBao{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
