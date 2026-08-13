package component

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWithoutValuesPreservesLifecycle(t *testing.T) {
	type key struct{}
	parent, cancel := context.WithTimeout(context.WithValue(t.Context(), key{}, "private"), time.Hour)
	ctx := WithoutValues(parent)
	if ctx.Value(key{}) != nil {
		t.Fatal("value was not suppressed")
	}
	if deadline, ok := ctx.Deadline(); !ok || deadline.IsZero() {
		t.Fatal("deadline was not preserved")
	}
	cancel()
	<-ctx.Done()
	if ctx.Err() != context.Canceled {
		t.Fatalf("Err() = %v, want context canceled", ctx.Err())
	}
}

func TestWithoutValuesPreservesOnlyRequestHeaders(t *testing.T) {
	type key struct{}
	headers := http.Header{"X-Tenant": {"tenant-a"}}
	parent := ContextWithRequestHeaders(context.WithValue(t.Context(), key{}, "private"), headers)
	headers.Set("X-Tenant", "mutated")

	ctx := WithoutValues(parent)
	if ctx.Value(key{}) != nil {
		t.Fatal("private value was not suppressed")
	}
	got := RequestHeadersFromContext(ctx)
	if got.Get("X-Tenant") != "tenant-a" {
		t.Fatalf("request header = %q, want tenant-a", got.Get("X-Tenant"))
	}
	got.Set("X-Tenant", "also-mutated")
	if again := RequestHeadersFromContext(ctx).Get("X-Tenant"); again != "tenant-a" {
		t.Fatalf("stored request header was mutated to %q", again)
	}
}

func TestDownstreamMetaRemovesFrontendProtocolMetadata(t *testing.T) {
	meta := mcp.Meta{
		mcp.MetaKeyProtocolVersion:    "2026-07-28",
		mcp.MetaKeyClientInfo:         map[string]any{"name": "frontend"},
		mcp.MetaKeyClientCapabilities: map[string]any{"tools": map[string]any{}},
		"application":                 "preserved",
	}
	clean := DownstreamMeta(meta)
	if len(clean) != 1 || clean["application"] != "preserved" {
		t.Fatalf("DownstreamMeta() = %#v, want only application metadata", clean)
	}
	if len(meta) != 4 {
		t.Fatal("DownstreamMeta mutated its input")
	}
}
