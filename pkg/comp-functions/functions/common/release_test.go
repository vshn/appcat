package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestGetObservedReleaseValues(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/01_release.yaml")

	values, err := GetObservedReleaseValues(svc, "release")
	assert.NoError(t, err)

	assert.NotEmpty(t, values)

}
