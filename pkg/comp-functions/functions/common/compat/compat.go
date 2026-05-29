// Package compat holds the pure service-version / AppCat-revision compatibility
// comparison logic. It has no kube client and no I/O so it can be shared by the
// composition-function detection step, the admission webhook, and maintenance.
package compat

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	sigsyaml "sigs.k8s.io/yaml"
)

// MatrixRow is one known-incompatibility constraint for a service: a service
// version range mapped to the AppCat revision range it is compatible with.
type MatrixRow struct {
	VersionRange        string `json:"versionRange"`
	CompatibleRevisions string `json:"compatibleRevisions"`
}

// Matrix maps a service name to its rows.
type Matrix map[string][]MatrixRow

// ParseMatrix parses the matrix.yaml document from the matrix ConfigMap.
// An empty input yields an empty Matrix (feature effectively disabled).
func ParseMatrix(raw []byte) (Matrix, error) {
	if len(raw) == 0 {
		return Matrix{}, nil
	}
	m := Matrix{}
	if err := sigsyaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Result is the outcome of a compatibility check.
type Result struct {
	Compatible bool
	Reason     string // why it is incompatible (empty when compatible)
	Action     string // actionable next steps for the customer
}

// Verdict reports whether runningVersion of serviceName is compatible with the
// AppCat revision couplet (e.g. "v3.72.2-v4.186.2"). Comparison uses the
// component (v3.x) half of the couplet, which is the strictly-monotonic axis.
//
// Fail-open: if the service has no rows, no row matches the version, or any
// value fails to parse, the result is Compatible (the matrix is an allowlist of
// known-incompatible constraints, not an exhaustive whitelist).
func Verdict(m Matrix, revision, serviceName, runningVersion string) Result {
	rows, ok := m[serviceName]
	if !ok || len(rows) == 0 {
		return Result{Compatible: true}
	}

	component, err := componentVersion(revision)
	if err != nil {
		return Result{Compatible: true}
	}

	version, err := semver.NewVersion(stripV(runningVersion))
	if err != nil {
		return Result{Compatible: true}
	}

	for _, row := range rows {
		versionConstraint, err := semver.NewConstraint(normalizeRange(row.VersionRange))
		if err != nil {
			continue
		}
		if !versionConstraint.Check(version) {
			continue
		}

		revisionConstraint, err := semver.NewConstraint(normalizeRange(row.CompatibleRevisions))
		if err != nil {
			continue
		}
		if revisionConstraint.Check(component) {
			return Result{Compatible: true}
		}

		return Result{
			Compatible: false,
			Reason: fmt.Sprintf(
				"%s version %s is incompatible with AppCat revision %s (requires %s)",
				serviceName, runningVersion, revision, row.CompatibleRevisions),
			Action: fmt.Sprintf(
				"Re-enable maintenance or unpin the version so %s can move to a compatible "+
					"AppCat revision (compatible revisions: %s).", serviceName, row.CompatibleRevisions),
		}
	}

	return Result{Compatible: true}
}

// componentVersion extracts and parses the component (first) half of a revision
// couplet such as "v3.72.2-v4.186.2".
func componentVersion(revision string) (*semver.Version, error) {
	if revision == "" {
		return nil, fmt.Errorf("empty revision")
	}
	first := strings.SplitN(revision, "-", 2)[0]
	return semver.NewVersion(stripV(first))
}

// stripV removes a single leading "v" so Masterminds parses "v3.72.2" cleanly.
func stripV(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "v")
}

// normalizeRange strips leading "v" from each whitespace-separated token in a
// constraint string (e.g. ">=v3.70.0" -> ">=3.70.0"). Masterminds handles
// wildcards ("16.x") and operators natively once the "v" is gone.
func normalizeRange(r string) string {
	tokens := strings.Fields(r)
	for i, tok := range tokens {
		for _, op := range []string{">=", "<=", ">", "<", "=", "~", "^"} {
			if strings.HasPrefix(tok, op) {
				tokens[i] = op + strings.TrimPrefix(tok[len(op):], "v")
				goto next
			}
		}
		tokens[i] = strings.TrimPrefix(tok, "v")
	next:
	}
	return strings.Join(tokens, " ")
}
