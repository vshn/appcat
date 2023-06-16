package vshnredis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
)

func TestReadValues(t *testing.T) {
	iof := loadRuntimeFromFile(t, "backup/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	assert.NoError(t, adjustHelmValues(context.TODO(), comp, iof))

}
