package runtime

import (
	"testing"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// TestRequiredResources verifies the manual proto walker can pull a Struct
// out of RunFunctionRequest.required_resources (proto field 8) when the
// SDK's generated message doesn't know about that field.
func TestRequiredResources(t *testing.T) {
	clusterStruct, err := structpb.NewStruct(map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name":      "postgresql",
			"namespace": "vshn-postgresql-foo",
		},
		"status": map[string]any{
			"phase": "Cluster in healthy state",
		},
	})
	if err != nil {
		t.Fatalf("build struct: %v", err)
	}

	structBytes, err := proto.Marshal(clusterStruct)
	if err != nil {
		t.Fatalf("marshal struct: %v", err)
	}

	// Resource { google.protobuf.Struct resource = 1; }
	var resource []byte
	resource = protowire.AppendTag(resource, 1, protowire.BytesType)
	resource = protowire.AppendBytes(resource, structBytes)

	// Resources { repeated Resource items = 1; }
	var resources []byte
	resources = protowire.AppendTag(resources, 1, protowire.BytesType)
	resources = protowire.AppendBytes(resources, resource)

	// Map entry { string key = 1; Resources value = 2; }
	var entry []byte
	entry = protowire.AppendTag(entry, 1, protowire.BytesType)
	entry = protowire.AppendString(entry, WatchedResourceKey)
	entry = protowire.AppendTag(entry, 2, protowire.BytesType)
	entry = protowire.AppendBytes(entry, resources)

	// RunFunctionRequest field 8 = required_resources (unknown to v0.4.0 SDK).
	var unknown []byte
	unknown = protowire.AppendTag(unknown, 8, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, entry)

	req := &fnv1.RunFunctionRequest{}
	req.ProtoReflect().SetUnknown(unknown)

	got, err := RequiredResources(req)
	if err != nil {
		t.Fatalf("RequiredResources: %v", err)
	}
	items := got[WatchedResourceKey]
	if len(items) != 1 {
		t.Fatalf("expected 1 item under %q, got %d", WatchedResourceKey, len(items))
	}

	m := items[0].AsMap()
	meta, _ := m["metadata"].(map[string]any)
	if meta["name"] != "postgresql" {
		t.Errorf("metadata.name = %v, want %q", meta["name"], "postgresql")
	}
	status, _ := m["status"].(map[string]any)
	if status["phase"] != "Cluster in healthy state" {
		t.Errorf("status.phase = %v, want %q", status["phase"], "Cluster in healthy state")
	}
}

// TestRequiredResources_Empty: no unknown fields means empty map.
func TestRequiredResources_Empty(t *testing.T) {
	req := &fnv1.RunFunctionRequest{}
	got, err := RequiredResources(req)
	if err != nil {
		t.Fatalf("RequiredResources: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}
