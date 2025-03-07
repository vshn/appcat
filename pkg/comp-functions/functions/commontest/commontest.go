package commontest

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/stretchr/testify/assert"
)

func LoadRuntimeFromFile(t assert.TestingT, file string) *runtime.ServiceRuntime {
	p, _ := filepath.Abs(".")
	before, _, _ := strings.Cut(p, "pkg")
	f, err := os.Open(before + "test/functions/" + file)
	assert.NoError(t, err)
	b1, err := os.ReadFile(f.Name())
	if err != nil {
		assert.FailNow(t, "can't get example")
	}
	req, err := BytesToRequest(b1)
	assert.NoError(t, err)

	config := &corev1.ConfigMap{}
	assert.NoError(t, request.GetInput(req, config))

	svc, err := runtime.NewServiceRuntime(logr.Discard(), *config, req)
	assert.NoError(t, err)

	return svc
}

func BytesToRequest(content []byte) (*xfnproto.RunFunctionRequest, error) {
	req := &xfnproto.RunFunctionRequest{}

	err := yaml.Unmarshal(content, req)
	if err != nil {
		return nil, err
	}

	return req, nil
}
