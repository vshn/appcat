package runtime

import (
	"context"
	"fmt"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// WatchedResourceKey is the required-resource key Crossplane uses to inject
// the resource that triggered a WatchOperation.
const WatchedResourceKey = "ops.crossplane.io/watched-resource"

// OperationHandler is the entry point for a Crossplane Operation function.
// It receives the raw RunFunctionRequest and a pre-populated response and is
// expected to mutate rsp.Desired.Resources and/or rsp.Results directly.
type OperationHandler func(ctx context.Context, req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error

var operationRegistry = map[string]OperationHandler{}

// RegisterOperation adds an operation handler, keyed by the `serviceName`
// value sent in the function input ConfigMap.
func RegisterOperation(name string, h OperationHandler) {
	operationRegistry[name] = h
}

// LookupOperation returns the registered handler for name, if any.
func LookupOperation(name string) (OperationHandler, bool) {
	h, ok := operationRegistry[name]
	return h, ok
}

// RequiredResources decodes the `required_resources` map (proto field 8 on
// RunFunctionRequest) directly from the request's unknown-fields bytes.
//
// The current function-sdk-go (v0.4.0) was generated before that field
// existed, so the SDK silently drops it as an unknown field. This walker
// pulls it back out without bumping the SDK.
//
// The returned map keys are the required-resource selectors; values are the
// list of resource objects Crossplane delivered for that selector.
func RequiredResources(req *fnv1.RunFunctionRequest) (map[string][]*structpb.Struct, error) {
	out := map[string][]*structpb.Struct{}
	unknown := req.ProtoReflect().GetUnknown()

	for len(unknown) > 0 {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return nil, fmt.Errorf("invalid tag in unknown fields: %w", protowire.ParseError(n))
		}
		unknown = unknown[n:]

		// Field 8 is map<string, Resources> required_resources.
		// Map entries are encoded as repeated length-delimited messages.
		if num != 8 || typ != protowire.BytesType {
			n = protowire.ConsumeFieldValue(num, typ, unknown)
			if n < 0 {
				return nil, fmt.Errorf("skip unknown field %d: %w", num, protowire.ParseError(n))
			}
			unknown = unknown[n:]
			continue
		}

		entry, n := protowire.ConsumeBytes(unknown)
		if n < 0 {
			return nil, fmt.Errorf("read map entry: %w", protowire.ParseError(n))
		}
		unknown = unknown[n:]

		key, items, err := decodeMapEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("decode required_resources entry: %w", err)
		}
		out[key] = append(out[key], items...)
	}
	return out, nil
}

// decodeMapEntry parses a single map<string, Resources> entry.
// Wire layout: { string key = 1; Resources value = 2; }
func decodeMapEntry(buf []byte) (string, []*structpb.Struct, error) {
	var key string
	var items []*structpb.Struct
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return "", nil, protowire.ParseError(n)
		}
		buf = buf[n:]
		switch {
		case num == 1 && typ == protowire.BytesType:
			s, n := protowire.ConsumeString(buf)
			if n < 0 {
				return "", nil, protowire.ParseError(n)
			}
			key = s
			buf = buf[n:]
		case num == 2 && typ == protowire.BytesType:
			b, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return "", nil, protowire.ParseError(n)
			}
			rs, err := decodeResources(b)
			if err != nil {
				return "", nil, err
			}
			items = append(items, rs...)
			buf = buf[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, buf)
			if n < 0 {
				return "", nil, protowire.ParseError(n)
			}
			buf = buf[n:]
		}
	}
	return key, items, nil
}

// decodeResources parses Resources { repeated Resource items = 1; }.
func decodeResources(buf []byte) ([]*structpb.Struct, error) {
	var out []*structpb.Struct
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		buf = buf[n:]
		if num == 1 && typ == protowire.BytesType {
			b, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			s, err := decodeResource(b)
			if err != nil {
				return nil, err
			}
			if s != nil {
				out = append(out, s)
			}
			buf = buf[n:]
			continue
		}
		n = protowire.ConsumeFieldValue(num, typ, buf)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		buf = buf[n:]
	}
	return out, nil
}

// decodeResource parses Resource { google.protobuf.Struct resource = 1; ... }
// and returns the Struct payload, or nil if absent.
func decodeResource(buf []byte) (*structpb.Struct, error) {
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		buf = buf[n:]
		if num == 1 && typ == protowire.BytesType {
			b, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			s := &structpb.Struct{}
			if err := proto.Unmarshal(b, s); err != nil {
				return nil, fmt.Errorf("unmarshal resource Struct: %w", err)
			}
			return s, nil
		}
		n = protowire.ConsumeFieldValue(num, typ, buf)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		buf = buf[n:]
	}
	return nil, nil
}
