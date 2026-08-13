package mmmcp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestStatefulSessionsIsolateAndReuseDownstreamSessions(t *testing.T) {
	fixture := testserver.New(t, testserver.Options{
		Stateful: true,
		Tools: []testserver.Tool{{
			Definition: &mcp.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}},
			Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			},
		}},
	})
	composite, err := mmmcp.New(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()

	first := connectLegacy(t, frontend, nil)
	second := connectLegacy(t, frontend, nil)
	for range 2 {
		if _, err := first.CallTool(t.Context(), &mcp.CallToolParams{Name: "echo"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := second.CallTool(t.Context(), &mcp.CallToolParams{Name: "echo"}); err != nil {
		t.Fatal(err)
	}
	if got := fixture.Discoveries(); got != 3 {
		t.Fatalf("downstream sessions = %d, want 3 (catalog plus two frontend runtimes)", got)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return fixture.Deletes() >= 1 })
	if _, err := second.CallTool(t.Context(), &mcp.CallToolParams{Name: "echo"}); err != nil {
		t.Fatalf("second session after first close: %v", err)
	}
	if got := fixture.Discoveries(); got != 3 {
		t.Fatalf("second frontend runtime was not reused: discoveries = %d", got)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStatefulEachPOSTSelectsCompleteContextConfig(t *testing.T) {
	defaultFixture := namedToolFixture(t, "default")
	firstFixture := namedToolFixture(t, "first")
	secondFixture := namedToolFixture(t, "second")
	defaultConfig := &config.Config{Servers: []config.Server{{Name: "default", URL: defaultFixture.URL}}}
	firstConfig := &config.Config{Servers: []config.Server{{Name: "first", URL: firstFixture.URL}}}
	secondConfig := &config.Config{Servers: []config.Server{{Name: "second", URL: secondFixture.URL}}}

	composite, err := mmmcp.New(t.Context(), defaultConfig, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	var currentMu sync.RWMutex
	var current *config.Config
	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentMu.RLock()
		cfg := current
		currentMu.RUnlock()
		if cfg != nil {
			r = r.WithContext(mmmcp.ContextWithConfig(r.Context(), cfg))
		}
		composite.HTTPHandler().ServeHTTP(w, r)
	}))
	defer frontend.Close()

	setConfig := func(cfg *config.Config) {
		currentMu.Lock()
		current = cfg
		currentMu.Unlock()
	}
	setConfig(firstConfig)
	session := connectLegacy(t, frontend, frontend.Client())
	defer session.Close()

	assertOnlyTool(t, session, "tool")
	assertToolText(t, session, "tool", "first")
	setConfig(nil)
	assertOnlyTool(t, session, "tool")
	assertToolText(t, session, "tool", "default")
	setConfig(secondConfig)
	assertOnlyTool(t, session, "tool")
	assertToolText(t, session, "tool", "second")
	if firstFixture.Discoveries() == 0 || secondFixture.Discoveries() == 0 {
		t.Fatalf("request catalogs were not compiled: first=%d second=%d", firstFixture.Discoveries(), secondFixture.Discoveries())
	}
}

func TestStatefulCancellationReachesComponent(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	fixture := testserver.New(t, testserver.Options{
		Stateful: true,
		Tools: []testserver.Tool{{
			Definition: &mcp.Tool{Name: "block", InputSchema: map[string]any{"type": "object"}},
			Handler: func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				close(started)
				<-ctx.Done()
				close(canceled)
				return nil, ctx.Err()
			},
		}},
	})
	composite, err := mmmcp.New(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()
	session := connectLegacy(t, frontend, nil)
	defer session.Close()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "block"})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("component call did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("component did not observe cancellation")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("frontend call unexpectedly succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("frontend call did not return after cancellation")
	}
}

func connectLegacy(t *testing.T, frontend *httptest.Server, client *http.Client) *mcp.ClientSession {
	t.Helper()
	if client == nil {
		client = frontend.Client()
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "legacy-test", Version: "1.0.0"}, nil)
	session, err := mcpClient.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: frontend.URL, HTTPClient: client, DisableStandaloneSSE: true}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestCurrentFrontendCallsToolOnStatefulHTTPComponent(t *testing.T) {
	fixture := testserver.New(t, testserver.Options{Stateful: true, Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "original", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invoked"}}}, nil
		},
	}}})
	composite, err := mmmcp.New(t.Context(), &config.Config{Servers: []config.Server{{Name: "legacy", URL: fixture.URL}}}, mmmcp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "current-test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: frontend.URL, HTTPClient: frontend.Client()}, &mcp.ClientSessionOptions{ProtocolVersion: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "original"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Content[0].(*mcp.TextContent).Text; got != "invoked" {
		t.Fatalf("tool result = %q, want invoked", got)
	}
	calls := fixture.Calls()
	if len(calls) != 1 || calls[0].Name != "original" {
		t.Fatalf("downstream calls = %+v, want original tool invoked once", calls)
	}
}

func namedToolFixture(t *testing.T, name string) *testserver.Server {
	t.Helper()
	return testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "tool", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: name}}}, nil
		},
	}}})
}

func assertOnlyTool(t *testing.T, session *mcp.ClientSession, want string) {
	t.Helper()
	result, err := session.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != want {
		t.Fatalf("tools = %v, want only %q", toolNameList(result.Tools), want)
	}
}

func assertToolText(t *testing.T, session *mcp.ClientSession, name, want string) {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Content[0].(*mcp.TextContent).Text; got != want {
		t.Fatalf("tool %q result = %q, want %q", name, got, want)
	}
}

func toolNameList(tools []*mcp.Tool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(fmt.Errorf("condition not met before timeout"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
