package vshnredis

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
)

func loadRuntimeFromFile(t assert.TestingT, file string) *runtime.Runtime {
	p, _ := filepath.Abs(".")
	before, _, _ := strings.Cut(p, "pkg")
	f, err := os.Open(before + "/test/transforms/vshnredis/" + file)
	assert.NoError(t, err)
	b1, err := os.ReadFile(f.Name())
	if err != nil {
		assert.FailNow(t, "can't get example")
	}
	funcIO, err := runtime.NewRuntime(context.Background(), b1)
	assert.NoError(t, err)

	return funcIO
}
