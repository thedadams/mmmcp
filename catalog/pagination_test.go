package catalog_test

import (
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/catalog"
	"github.com/obot-platform/mmmcp/component"
	"github.com/obot-platform/mmmcp/config"
)

func TestCatalogPaginationIsSortedOpaqueAndFamilyBound(t *testing.T) {
	discoverer := featureDiscoverer{features: map[string]*component.Features{"fixture": {
		Tools:   []*mcp.Tool{{Name: "z", InputSchema: map[string]any{"type": "object"}}, {Name: "a", InputSchema: map[string]any{"type": "object"}}, {Name: "m", InputSchema: map[string]any{"type": "object"}}},
		Prompts: []*mcp.Prompt{{Name: "z"}, {Name: "a"}},
	}}}
	compiled, err := catalog.Compile(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: "https://example.invalid"}}}, discoverer)
	if err != nil {
		t.Fatal(err)
	}
	first, cursor, err := compiled.PageTools("", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].Name != "a" || first[1].Name != "m" || cursor == "" {
		t.Fatalf("first page = %+v, cursor = %q", first, cursor)
	}
	second, next, err := compiled.PageTools(cursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Name != "z" || next != "" {
		t.Fatalf("second page = %+v, cursor = %q", second, next)
	}
	_, _, err = compiled.PagePrompts(cursor, 2)
	assertInvalidParams(t, err)
	_, _, err = compiled.PageTools("not-base64", 2)
	assertInvalidParams(t, err)

	other, err := catalog.Compile(t.Context(), &config.Config{Servers: []config.Server{{Name: "other", URL: "https://example.invalid"}}}, featureDiscoverer{features: map[string]*component.Features{"other": {Tools: []*mcp.Tool{{Name: "a", InputSchema: map[string]any{"type": "object"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = other.PageTools(cursor, 2)
	assertInvalidParams(t, err)
}

func assertInvalidParams(t *testing.T, err error) {
	t.Helper()
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("error = %v, want Invalid Params", err)
	}
}
