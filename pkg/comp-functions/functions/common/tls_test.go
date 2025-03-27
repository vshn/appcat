package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestCreateTLSCertsNilOptions(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "empty.yaml")

	_, err := CreateTLSCerts(context.TODO(), "mytest", "mytest", svc, nil)
	assert.NoError(t, err)
}
