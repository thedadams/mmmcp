package stdio

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/config"
)

func TestPaginateTreatsMethodNotFoundAsEmptyList(t *testing.T) {
	for _, method := range []string{"tools/list", "prompts/list", "resources/list", "resources/templates/list"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			err := paginate("fixture", method, func(string) (string, error) {
				calls++
				return "", fmt.Errorf("list response: %w", &jsonrpc.Error{
					Code:    jsonrpc.CodeMethodNotFound,
					Message: "Method not found",
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("page calls = %d, want 1", calls)
			}
		})
	}
}

func TestSuccessfulOneOffReturnsBeforeStubbornProcessCleanup(t *testing.T) {
	const terminateDuration = 500 * time.Millisecond
	factory := NewFactory(Options{
		LookupEnv:         func(string) (string, bool) { return "", false },
		TerminateDuration: terminateDuration,
	})
	server := config.Server{
		Name:    "stubborn",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestStubbornStdioHelperProcess"},
		Env:     map[string]string{"MMMCP_STUBBORN_HELPER": "1"},
	}

	started := time.Now()
	result, err := factory.CallTool(t.Context(), server, &mcp.CallToolParams{Name: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed >= terminateDuration/2 {
		t.Fatalf("tool call waited for process cleanup: %v", elapsed)
	}
	if got := result.Content[0].(*mcp.TextContent).Text; got != "invoked" {
		t.Fatalf("tool result = %q, want invoked", got)
	}

	closeStarted := time.Now()
	if err := factory.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(closeStarted); elapsed < terminateDuration/2 {
		t.Fatalf("factory close did not wait for cleanup: %v", elapsed)
	}
	if _, err := factory.CallTool(t.Context(), server, &mcp.CallToolParams{Name: "ok"}); err == nil {
		t.Fatal("closed factory accepted a one-off operation")
	}
}

func TestStubbornStdioHelperProcess(t *testing.T) {
	if os.Getenv("MMMCP_STUBBORN_HELPER") != "1" {
		return
	}
	signal.Ignore(syscall.SIGTERM)
	server := mcp.NewServer(&mcp.Implementation{Name: "stubborn-helper", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "ok", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "invoked"}}}, nil
	})
	_ = server.Run(t.Context(), &mcp.StdioTransport{})
	for {
		time.Sleep(time.Hour)
	}
}
