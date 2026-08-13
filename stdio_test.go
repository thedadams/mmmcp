package mmmcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/storage"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestRunPersistentUsesProcessScopedContextConfig(t *testing.T) {
	defaultFixture := testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "default", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	}}})
	requestFixture := testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "request", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	}}})

	composite, err := New(t.Context(), &config.Config{Servers: []config.Server{{Name: "default", URL: defaultFixture.URL}}}, Options{
		Storage: storage.Options{DataDirectory: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	runCtx, cancel := context.WithCancel(ContextWithConfig(t.Context(), &config.Config{
		Servers: []config.Server{{Name: "selected", URL: requestFixture.URL}},
	}))
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- composite.runPersistent(runCtx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "request" {
		t.Fatalf("stdio tools = %+v", listed.Tools)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-runDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("RunStdio error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("persistent server did not stop after client close")
	}
}

func TestCloseStopsPersistentFrontend(t *testing.T) {
	fixture := testserver.New(t, testserver.Options{Tools: []testserver.Tool{{
		Definition: &mcp.Tool{Name: "tool", InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	}}})
	composite, err := New(t.Context(), &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}, Options{
		Storage: storage.Options{DataDirectory: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	runDone := make(chan error, 1)
	go func() { runDone <- composite.runPersistent(t.Context(), serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-close-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- composite.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Composite.Close did not stop persistent frontend")
	}
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("persistent frontend did not return")
	}
	_ = session.Close()
}
