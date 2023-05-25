package runtime

import (
	"context"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

type Result interface {
	Resolve() v1alpha1.Result
}

type result v1alpha1.Result

// NewWarning returns a warning Result from a message string. The entire
// Composition will run to completion but warning events and debug logs
// associated with the composite resource will be emitted.
func NewWarning(ctx context.Context, msg string) Result {
	controllerruntime.LoggerFrom(ctx).Info(msg)
	return result{
		Severity: v1alpha1.SeverityWarning,
		Message:  msg,
	}
}

// NewFatalErr returns a fatal Result from a message string and an error. The result is fatal,
// subsequent Composition Functions may run, but the Composition Function pipeline run
// will be considered a failure and the first error will be returned.
func NewFatalErr(ctx context.Context, msg string, err error) Result {
	controllerruntime.LoggerFrom(ctx).Error(err, msg)
	return result{
		Severity: v1alpha1.SeverityFatal,
		Message:  msg,
	}
}

// NewFatal returns a fatal Result from a message string. The result is fatal,
// subsequent Composition Functions may run, but the Composition Function
// pipeline run will be considered a failure and the first error will be returned.
func NewFatal(ctx context.Context, msg string) Result {
	controllerruntime.LoggerFrom(ctx).Info(msg)
	return result{
		Severity: v1alpha1.SeverityFatal,
		Message:  msg,
	}
}

// NewNormal results are emitted as normal events and debug logs associated
// with the composite resource.
func NewNormal() Result {
	return result{
		Severity: v1alpha1.SeverityNormal,
		Message:  "function ran successfully",
	}
}

// Resolve returns the wrapped object v1alpha1.Result from crossplane
func (r result) Resolve() v1alpha1.Result {
	return v1alpha1.Result(r)
}
