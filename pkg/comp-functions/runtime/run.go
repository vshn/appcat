package runtime

import (
	"context"
	"fmt"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/yaml"
)

// Exec reads FunctionIO from stdin and return the desired state via transform function
func Exec(ctx context.Context, runtime *Runtime, transform Transform) error {

	res := transform.TransformFunc(ctx, runtime).Resolve()
	res.Message = fmt.Sprintf("Function %s result: %s", transform.Name, res.Message)

	runtime.io.Results = append(runtime.io.Results, res)

	runtime.io.Desired.Composite.Resource.Raw = runtime.Desired.composite.Resource.Raw
	runtime.io.Desired.Composite.ConnectionDetails = runtime.Desired.composite.ConnectionDetails

	runtime.io.Desired.Resources = make([]xfnv1alpha1.DesiredResource, len(runtime.Desired.resources))
	for i, r := range runtime.Desired.resources {
		runtime.io.Desired.Resources[i] = r.GetDesiredResource()
	}

	return nil
}

// printFunctionIO prints the whole FunctionIO to stdout, so Crossplane can
// pick it up again.
func printFunctionIO(iof *xfnv1alpha1.FunctionIO, log logr.Logger) ([]byte, error) {
	log.V(1).Info("Marshalling FunctionIO")
	fnc, err := yaml.Marshal(iof)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal function io: %w", err)
	}
	return fnc, nil
}

func RunCommand(ctx context.Context, input []byte, transforms []Transform) ([]byte, error) {
	log := logr.FromContextOrDiscard(ctx)

	log.V(1).Info("Creating new runtime")
	funcIO, err := NewRuntime(ctx, input)
	if err != nil {
		return []byte{}, err
	}

	ctx, log, err = prepareFuncLogger(ctx, log, funcIO)
	if err != nil {
		return []byte{}, err
	}

	for _, function := range transforms {
		log.Info("Starting function", "name", function.Name)
		err = Exec(ctx, funcIO, function)
		if err != nil {
			return []byte{}, err
		}
	}

	return printFunctionIO(&funcIO.io, log)
}

// prepareFuncLogger reads some metadata from the funcIO and uses it to add it to the logger.
func prepareFuncLogger(ctx context.Context, log logr.Logger, iof *Runtime) (context.Context, logr.Logger, error) {

	// missusing the xkube object a bit here...
	// I'm only interested in the metadata
	meta := &xkube.Object{}
	if err := iof.Desired.GetComposite(ctx, meta); err != nil {
		return ctx, log, err
	}

	log = log.WithValues(
		"compositeName",
		meta.GetName(),
	)

	ctx = logr.NewContext(ctx, log)

	return ctx, log, nil
}
