package release

import (
	"context"

	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/compat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// serviceVersionField maps a composite kind (without leading X) to the compat
// service name and the spec field path holding the running service version.
var serviceVersionField = map[string]struct {
	service string
	path    []string
}{
	"VSHNPostgreSQL": {"postgresql", []string{"spec", "parameters", "service", "majorVersion"}},
	"VSHNRedis":      {"redis", []string{"spec", "parameters", "service", "version"}},
	"VSHNMariaDB":    {"mariadb", []string{"spec", "parameters", "service", "version"}},
	"VSHNKeycloak":   {"keycloak", []string{"spec", "parameters", "service", "version"}},
	"VSHNNextcloud":  {"nextcloud", []string{"spec", "parameters", "service", "version"}},
	"VSHNForgejo":    {"forgejo", []string{"spec", "parameters", "service", "majorVersion"}},
}

// filterRevisionsByCompatibility keeps only revision labels whose component
// half is compatible with the given running service version per the matrix.
// Fail-open: an empty matrix or a service with no rows keeps every revision.
func filterRevisionsByCompatibility(revisionLabels []string, m compat.Matrix, serviceName, runningVersion string) []string {
	if len(m) == 0 {
		return revisionLabels
	}
	var keep []string
	for _, label := range revisionLabels {
		if compat.Verdict(m, label, serviceName, runningVersion).Compatible {
			keep = append(keep, label)
		}
	}
	return keep
}

// loadMatrixAndVersion reads the compat matrix ConfigMap and the running service
// version for the composite this handler targets. It fails open: any missing
// matrix, unknown kind, or unreadable composite yields an empty matrix so the
// compatibility filter becomes a no-op.
func (vh *DefaultVersionHandler) loadMatrixAndVersion(ctx context.Context) (compat.Matrix, string, string) {
	mapping, ok := serviceVersionField[removeLeadingX(vh.ownerKind)]
	if !ok {
		return compat.Matrix{}, "", ""
	}

	cm := &corev1.ConfigMap{}
	if err := vh.client.Get(ctx, types.NamespacedName{
		Name:      compat.MatrixConfigMapName,
		Namespace: compat.MatrixNamespace,
	}, cm); err != nil {
		return compat.Matrix{}, "", ""
	}
	matrix, err := compat.ParseMatrix([]byte(cm.Data["matrix.yaml"]))
	if err != nil {
		return compat.Matrix{}, "", ""
	}

	comp := composite.New()
	comp.SetGroupVersionKind(vh.getCompositeGVK())
	if err := vh.client.Get(ctx, types.NamespacedName{Name: vh.compositeName}, comp); err != nil {
		return compat.Matrix{}, "", ""
	}
	version, _, err := unstructured.NestedString(comp.Object, mapping.path...)
	if err != nil || version == "" {
		return compat.Matrix{}, "", ""
	}

	return matrix, version, mapping.service
}
