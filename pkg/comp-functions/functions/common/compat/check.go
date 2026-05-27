package compat

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	// MatrixNamespace is where component-appcat renders the matrix ConfigMap.
	MatrixNamespace = "syn-appcat"
	// MatrixConfigMapName is the matrix ConfigMap name.
	MatrixConfigMapName = "appcat-compat-matrix"
	// MatrixObserverName is the desired-resource key used to observe the CM.
	MatrixObserverName = "appcat-compat-matrix-observer"
	// ConditionTypeVersionIncompatible is the condition type set on the claim.
	ConditionTypeVersionIncompatible = "VersionIncompatible"
)

// RunCompatCheck observes the matrix ConfigMap, evaluates compatibility of
// runningVersion against the instance's own revision couplet, and:
//   - on incompatible: emits a warning Result (event on composite + claim) and
//     calls setCondition with a True VersionIncompatible condition;
//   - on compatible: calls setCondition with a False condition;
//   - if the matrix is not yet observed: returns nil (retried next reconcile).
//
// It never returns a fatal Result for a missing/unparseable matrix (fail-open);
// it only returns fatal on an internal observe-setup error.
func RunCompatCheck(
	ctx context.Context,
	svc *runtime.ServiceRuntime,
	serviceName, runningVersion, revision string,
	setCondition func(vshnv1.Condition),
) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	observerCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MatrixConfigMapName,
			Namespace: MatrixNamespace,
		},
	}
	if err := svc.SetDesiredKubeObject(observerCM, MatrixObserverName,
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
	); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set matrix observer: %w", err))
	}

	observedCM := &corev1.ConfigMap{}
	if err := svc.GetObservedKubeObject(observedCM, MatrixObserverName); err != nil {
		if err == runtime.ErrNotFound {
			log.Info("compat matrix not observed yet, retrying next reconcile")
			return nil
		}
		// Fail-open on any other read problem.
		log.Info("cannot read compat matrix, treating as compatible", "error", err.Error())
		return nil
	}

	matrix, err := ParseMatrix([]byte(observedCM.Data["matrix.yaml"]))
	if err != nil {
		svc.AddResult(runtime.NewWarningResult(
			fmt.Sprintf("compat matrix is unparseable, skipping version compatibility check: %v", err)))
		return nil
	}

	result := Verdict(matrix, revision, serviceName, runningVersion)
	if result.Compatible {
		setCondition(vshnv1.Condition{
			Type:               ConditionTypeVersionIncompatible,
			Status:             metav1.ConditionFalse,
			Reason:             "Compatible",
			Message:            "Service version is compatible with the current AppCat revision.",
			LastTransitionTime: metav1.Now(),
		})
		return nil
	}

	svc.AddResult(runtime.NewWarningResult(
		fmt.Sprintf("Incompatibility Warning: %s %s", result.Reason, result.Action)))
	setCondition(vshnv1.Condition{
		Type:               ConditionTypeVersionIncompatible,
		Status:             metav1.ConditionTrue,
		Reason:             "VersionIncompatible",
		Message:            result.Reason + " " + result.Action,
		LastTransitionTime: metav1.Now(),
	})
	return nil
}

// UpsertCondition replaces an existing condition of the same Type or appends it.
func UpsertCondition(conditions []vshnv1.Condition, c vshnv1.Condition) []vshnv1.Condition {
	for i := range conditions {
		if conditions[i].Type == c.Type {
			conditions[i] = c
			return conditions
		}
	}
	return append(conditions, c)
}
