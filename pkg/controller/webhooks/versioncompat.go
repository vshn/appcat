package webhooks

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	corev1 "k8s.io/api/core/v1"
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

// currentRevisionFromStatus returns the AppCat revision the instance is actually
// running on, recorded by the composition function in status. Empty when the
// instance has not reconciled yet (e.g. on create).
func currentRevisionFromStatus(obj runtime.Object) string {
	switch o := obj.(type) {
	case *vshnv1.VSHNPostgreSQL:
		return o.Status.CurrentRevision
	case *vshnv1.VSHNRedis:
		return o.Status.CurrentRevision
	case *vshnv1.VSHNMariaDB:
		return o.Status.CurrentRevision
	case *vshnv1.VSHNKeycloak:
		return o.Status.CurrentRevision
	case *vshnv1.VSHNNextcloud:
		return o.Status.CurrentRevision
	case *vshnv1.VSHNForgejo:
		return o.Status.CurrentRevision
	default:
		return ""
	}
}

// loadCompatMatrix reads the matrix ConfigMap and returns the cluster's current
// AppCat revision couplet and the parsed matrix. ok=false on any read/parse
// problem so callers fail open.
func loadCompatMatrix(ctx context.Context, c client.Client) (revision string, matrix compat.Matrix, ok bool) {
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      compat.MatrixConfigMapName,
		Namespace: compat.MatrixNamespace,
	}, cm); err != nil {
		return "", nil, false
	}
	matrix, err := compat.ParseMatrix([]byte(cm.Data["matrix.yaml"]))
	if err != nil {
		return "", nil, false
	}
	return cm.Data["currentRevision"], matrix, true
}

// checkVersionCompat returns a Forbidden field error if obj's proposed version
// is incompatible with the cluster's current AppCat revision. It fails open
// (returns nil) for unknown types or a missing/unparseable matrix.
func checkVersionCompat(ctx context.Context, c client.Client, obj runtime.Object) *field.Error {
	service, version, ok := versionExtractor(obj)
	if !ok {
		return nil
	}
	cmRevision, matrix, ok := loadCompatMatrix(ctx, c)
	if !ok {
		return nil
	}
	// Prefer the instance's actual running revision (recorded in status by the
	// composition function); fall back to the cluster's latest revision from the
	// matrix CM for instances that have not reconciled yet (e.g. on create).
	revision := cmRevision
	if sr := currentRevisionFromStatus(obj); sr != "" {
		revision = sr
	}
	result := compat.Verdict(matrix, revision, service, version)
	if result.Compatible {
		return nil
	}
	return field.Forbidden(
		field.NewPath("spec", "parameters", "service", "version"),
		result.Reason+" "+result.Action,
	)
}
