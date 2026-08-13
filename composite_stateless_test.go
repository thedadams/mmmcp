package mmmcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestCompositeStatelessToolEndToEnd(t *testing.T) {
	cancelStarted := make(chan struct{})
	cancelObserved := make(chan struct{})
	fixture := testserver.New(t, testserver.Options{
		PageSize: 1,
		Tools: []testserver.Tool{
			{
				Definition: &mcp.Tool{Name: "zeta", Description: "last tool", InputSchema: map[string]any{"type": "object"}},
				Handler: func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(req.Params.Arguments)}}}, nil
				},
			},
			{
				Definition: &mcp.Tool{Name: "cancel", Description: "blocks until canceled", InputSchema: map[string]any{"type": "object"}},
				Handler: func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					close(cancelStarted)
					<-ctx.Done()
					close(cancelObserved)
					return nil, ctx.Err()
				},
			},
		},
	})
	cfg := &config.Config{Servers: []config.Server{{Name: "example component", Prefix: "demo", URL: fixture.URL}}}
	composite, err := mmmcp.New(t.Context(), cfg, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()

	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: frontend.URL, HTTPClient: frontend.Client(), DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 2 || listed.Tools[0].Name != "demo__cancel" || listed.Tools[1].Name != "demo__zeta" {
		t.Fatalf("listed tools = %+v", listed.Tools)
	}
	if listed.Tools[1].Description != "last tool" {
		t.Fatalf("description = %q", listed.Tools[1].Description)
	}

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "demo__zeta", Arguments: map[string]any{"value": "ok"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Content[0].(*mcp.TextContent).Text; got != `{"value":"ok"}` {
		t.Fatalf("tool result = %q", got)
	}
	calls := fixture.Calls()
	if len(calls) != 1 || calls[0].Name != "zeta" {
		t.Fatalf("component calls = %+v", calls)
	}
	var arguments map[string]any
	if err := json.Unmarshal(calls[0].Arguments, &arguments); err != nil || arguments["value"] != "ok" {
		t.Fatalf("component arguments = %s, error = %v", calls[0].Arguments, err)
	}

	callCtx, cancel := context.WithCancel(t.Context())
	callDone := make(chan error, 1)
	go func() {
		_, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "demo__cancel"})
		callDone <- err
	}()
	select {
	case <-cancelStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("component call did not start")
	}
	cancel()
	select {
	case err := <-callDone:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled call error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("frontend call did not cancel")
	}
	select {
	case <-cancelObserved:
	case <-time.After(5 * time.Second):
		t.Fatal("component did not observe cancellation")
	}

	if got := fixture.Discoveries(); got != 3 {
		t.Fatalf("downstream sessions = %d, want 3 (startup discovery plus two calls)", got)
	}
}

func TestCompositeCloseIsIdempotentAndRejectsRequests(t *testing.T) {
	fixture := testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "tool", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{}}, nil
		},
	}}})
	composite, err := mmmcp.New(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := composite.Close(); err != nil {
		t.Fatal(err)
	}
	if err := composite.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.invalid/mcp", nil)
	composite.HTTPHandler().ServeHTTP(recorder, request)
	if recorder.Code != 503 {
		t.Fatalf("status after Close = %d, want 503", recorder.Code)
	}
}
