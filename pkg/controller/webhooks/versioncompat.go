package webhooks

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// versionExtractor returns (serviceName, version) for a known version-bearing
// service type, and ok=false for any object we do not range on.
func versionExtractor(obj runtime.Object) (service, version string, ok bool) {
	switch o := obj.(type) {
	case *vshnv1.VSHNPostgreSQL:
		return "postgresql", o.Spec.Parameters.Service.MajorVersion, true
	case *vshnv1.VSHNRedis:
		return "redis", o.Spec.Parameters.Service.Version, true
	case *vshnv1.VSHNMariaDB:
		return "mariadb", o.Spec.Parameters.Service.Version, true
	case *vshnv1.VSHNKeycloak:
		return "keycloak", o.Spec.Parameters.Service.Version, true
	case *vshnv1.VSHNNextcloud:
		return "nextcloud", o.Spec.Parameters.Service.Version, true
	case *vshnv1.VSHNForgejo:
		return "forgejo", o.Spec.Parameters.Service.MajorVersion, true
	default:
		return "", "", false
	}
}

// pinnedRevision returns the AppCat revision couplet the instance is currently
// pinned to, read from the live object's compositionRevisionSelector. The typed
// admission object cannot carry that selector (the appcat specs embed the
// managed-resource spec, not the composite spec), so we re-read the live object
// as unstructured. Empty when the instance is unpinned (release management off),
// in which case the caller fails open.
func pinnedRevision(ctx context.Context, c client.Client, obj runtime.Object) string {
	mo, ok := obj.(metav1.Object)
	if !ok {
		return ""
	}
	gvks, _, err := c.Scheme().ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		return ""
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvks[0])
	if err := c.Get(ctx, types.NamespacedName{Namespace: mo.GetNamespace(), Name: mo.GetName()}, u); err != nil {
		return ""
	}
	return revisionFromUnstructured(u)
}

// revisionFromUnstructured extracts the pinned revision couplet from an
// unstructured composite/claim's spec.compositionRevisionSelector.matchLabels.
// Empty when no selector (or the revision label) is present.
func revisionFromUnstructured(u *unstructured.Unstructured) string {
	matchLabels, found, err := unstructured.NestedStringMap(u.Object, "spec", "compositionRevisionSelector", "matchLabels")
	if err != nil || !found {
		return ""
	}
	return matchLabels[release.RevisionLabel]
}

// verdictFieldError turns an incompatible Verdict into a Forbidden field error,
// or nil when compatible.
func verdictFieldError(matrix compat.Matrix, revision, service, version string) *field.Error {
	result := compat.Verdict(matrix, revision, service, version)
	if result.Compatible {
		return nil
	}
	return field.Forbidden(
		field.NewPath("spec", "parameters", "service", "version"),
		result.Reason+" "+result.Action,
	)
}

// loadCompatMatrix reads and parses the matrix ConfigMap. ok=false on any
// read/parse problem so callers fail open.
func loadCompatMatrix(ctx context.Context, c client.Client) (matrix compat.Matrix, ok bool) {
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      compat.MatrixConfigMapName,
		Namespace: compat.MatrixNamespace,
	}, cm); err != nil {
		return nil, false
	}
	matrix, err := compat.ParseMatrix([]byte(cm.Data["matrix.yaml"]))
	if err != nil {
		return nil, false
	}
	return matrix, true
}

// checkVersionCompat returns a Forbidden field error if obj's proposed version is
// incompatible with the revision the instance is currently pinned to. It fails
// open (returns nil) for unknown types, an unpinned instance, or a
// missing/unparseable matrix.
func checkVersionCompat(ctx context.Context, c client.Client, obj runtime.Object) *field.Error {
	service, version, ok := versionExtractor(obj)
	if !ok {
		return nil
	}
	revision := pinnedRevision(ctx, c, obj)
	if revision == "" {
		return nil
	}
	matrix, ok := loadCompatMatrix(ctx, c)
	if !ok {
		return nil
	}
	return verdictFieldError(matrix, revision, service, version)
}
