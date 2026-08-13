package component_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/component"
)

func TestStatelessClientRejectsDirectRootsRequest(t *testing.T) {
	componentServer := mcp.NewServer(&mcp.Implementation{Name: "component", Version: "test"}, nil)
	componentServer.AddTool(&mcp.Tool{Name: "roots", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, err := req.Session.ListRoots(ctx, nil) //nolint:staticcheck // Exercise roots behavior for a legacy MCP version.
		return nil, err
	})
	client := component.NewClient(&mcp.Implementation{Name: "mmmcp", Version: "test"}, component.Callbacks{Component: "fixture"})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverErr := make(chan error, 1)
	go func() { serverErr <- componentServer.Run(t.Context(), serverTransport) }()
	t.Cleanup(func() {
		if err := <-serverErr; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("component server: %v", err)
		}
	})
	session, err := client.Connect(t.Context(), clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	_, err = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "roots"})
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("direct callback error = %v, want method-not-found", err)
	}
}
