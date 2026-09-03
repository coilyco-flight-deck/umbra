package otelsink

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// resourceOf builds a resource from semconv attribute values, keeping the
// variadic any-typed call in NewProvider off the semconv import surface.
func resourceOf(attrs []any) *resource.Resource {
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if kv, ok := a.(attribute.KeyValue); ok {
			kvs = append(kvs, kv)
		}
	}
	return resource.NewWithAttributes(semconv.SchemaURL, kvs...)
}
