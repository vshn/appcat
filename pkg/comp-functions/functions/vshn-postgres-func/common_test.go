package vshnpostgres

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"

	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"

	"github.com/stretchr/testify/assert"
)

func loadRuntimeFromFile(t assert.TestingT, file string) *runtime.Runtime {
	p, _ := filepath.Abs(".")
	before, _, _ := strings.Cut(p, "pkg")
	f, err := os.Open(before + "/test/transforms/vshn-postgres/" + file)
	assert.NoError(t, err)
	b1, err := os.ReadFile(f.Name())
	if err != nil {
		assert.FailNow(t, "can't get example")
	}
	funcIO, err := runtime.NewRuntime(context.Background(), b1)
	assert.NoError(t, err)

	return funcIO
}

func getFunctionIo(funcIO *runtime.Runtime) xfnv1alpha1.FunctionIO {
	field := reflect.ValueOf(funcIO).Elem().FieldByName("io")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(xfnv1alpha1.FunctionIO)
}
