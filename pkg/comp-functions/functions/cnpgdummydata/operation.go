// Package cnpgdummydata is a quick-and-dirty PoC of a Crossplane Operation
// function. Triggered by a WatchOperation on CNPG Cluster resources; once a
// cluster reaches "Cluster in healthy state", the handler emits a Job that
// connects via psql and inserts a dummy row.
package cnpgdummydata

import (
	"context"
	"fmt"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const serviceName = "cnpg-dummy-data"

// CNPG sets this exact phase string when the primary instance is up.
const cnpgHealthyPhase = "Cluster in healthy state"

func init() {
	runtime.RegisterOperation(serviceName, Handle)
}

// Handle is the operation entry point.
func Handle(_ context.Context, req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error {
	required, err := runtime.RequiredResources(req)
	if err != nil {
		return fmt.Errorf("decode required_resources: %w", err)
	}
	items := required[runtime.WatchedResourceKey]
	if len(items) == 0 {
		response.Warning(rsp, fmt.Errorf("no watched resource at %q", runtime.WatchedResourceKey))
		return nil
	}

	cluster := items[0].AsMap()
	name, _ := nestedString(cluster, "metadata", "name")
	ns, _ := nestedString(cluster, "metadata", "namespace")
	phase, _ := nestedString(cluster, "status", "phase")

	if phase != cnpgHealthyPhase {
		response.Normal(rsp, fmt.Sprintf("cluster %s/%s phase=%q; skipping", ns, name, phase))
		return nil
	}

	job, err := buildJob(ns, name)
	if err != nil {
		return fmt.Errorf("build job: %w", err)
	}

	if rsp.Desired == nil {
		rsp.Desired = &fnv1.State{}
	}
	if rsp.Desired.Resources == nil {
		rsp.Desired.Resources = map[string]*fnv1.Resource{}
	}
	rsp.Desired.Resources["dummy-data-job"] = &fnv1.Resource{Resource: job}

	response.Normal(rsp, fmt.Sprintf("scheduled dummy-data Job for %s/%s", ns, name))
	return nil
}

func buildJob(ns, cluster string) (*structpb.Struct, error) {
	sql := `CREATE TABLE IF NOT EXISTS dummy(id serial primary key, msg text, created timestamptz default now());
INSERT INTO dummy(msg) SELECT 'hello from crossplane operation' WHERE NOT EXISTS (SELECT 1 FROM dummy);`

	obj := map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":      "dummy-data-" + cluster,
			"namespace": ns,
			"labels": map[string]any{
				"app.kubernetes.io/managed-by": "crossplane-operation-poc",
				"cnpg.io/cluster":              cluster,
			},
		},
		"spec": map[string]any{
			"backoffLimit":            float64(5),
			"ttlSecondsAfterFinished": float64(3600),
			"template": map[string]any{
				"spec": map[string]any{
					"restartPolicy": "OnFailure",
					"containers": []any{
						map[string]any{
							"name":  "psql",
							"image": "ghcr.io/cloudnative-pg/postgresql:16",
							"env": []any{
								map[string]any{
									"name":  "PGHOST",
									"value": fmt.Sprintf("%s-rw.%s.svc.cluster.local", cluster, ns),
								},
								map[string]any{
									"name": "PGUSER",
									"valueFrom": map[string]any{
										"secretKeyRef": map[string]any{
											"name": cluster + "-superuser",
											"key":  "username",
										},
									},
								},
								map[string]any{
									"name": "PGPASSWORD",
									"valueFrom": map[string]any{
										"secretKeyRef": map[string]any{
											"name": cluster + "-superuser",
											"key":  "password",
										},
									},
								},
								map[string]any{"name": "PGDATABASE", "value": "app"},
								map[string]any{"name": "SQL", "value": sql},
							},
							"command": []any{"sh", "-c", `psql -v ON_ERROR_STOP=1 -c "$SQL"`},
						},
					},
				},
			},
		},
	}

	return structpb.NewStruct(obj)
}

func nestedString(m map[string]any, path ...string) (string, bool) {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = mm[p]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}
