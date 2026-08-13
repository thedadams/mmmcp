package mmmcp_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestCompositePreservesModernMultiRoundTripFields(t *testing.T) {
	fixture := testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "interactive", InputSchema: map[string]any{"type": "object"}},
		Handler: func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if req.Params.RequestState == "state-1" {
				if _, ok := req.Params.InputResponses["question"]; !ok {
					t.Fatal("retry omitted input response")
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "complete"}}}, nil
			}
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{"question": &mcp.ElicitParams{Message: "continue?"}},
				RequestState:  "state-1",
			}, nil
		},
	}}})
	composite, err := mmmcp.New(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "mrtr-test", Version: "test"}, &mcp.ClientOptions{MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true}})
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: frontend.URL, HTTPClient: frontend.Client(), DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	first, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "interactive"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.NeedsInput() || first.RequestState != "state-1" || first.InputRequests["question"] == nil {
		t.Fatalf("input-required result = %+v", first)
	}
	second, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:           "interactive",
		RequestState:   first.RequestState,
		InputResponses: mcp.InputResponseMap{"question": &mcp.ElicitResult{Action: "accept", Content: map[string]any{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := second.Content[0].(*mcp.TextContent).Text; got != "complete" {
		t.Fatalf("retry result = %q", got)
	}
}
