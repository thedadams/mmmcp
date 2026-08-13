package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if len(os.Args) < 2 {
		fatal(errors.New("usage: imagecheck fixture|client"))
	}
	var err error
	switch os.Args[1] {
	case "fixture":
		err = runFixture(os.Args[2:])
	case "client":
		err = runClient(os.Args[2:])
	default:
		err = fmt.Errorf("unknown mode %q", os.Args[1])
	}
	if err != nil {
		fatal(err)
	}
}

func runFixture(args []string) error {
	flags := flag.NewFlagSet("fixture", flag.ContinueOnError)
	listen := flags.String("listen", "0.0.0.0:0", "fixture listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "image-fixture", Version: "1.0.0"}, nil)
	server.AddTool(&mcp.Tool{Name: "echo", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "image-ok"}}}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	if _, err := fmt.Fprintln(os.Stdout, listener.Addr().String()); err != nil {
		return err
	}
	return http.Serve(listener, handler)
}

func runClient(args []string) error {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)
	endpoint := flags.String("endpoint", "", "composite MCP endpoint")
	toolName := flags.String("tool", "fixture__echo", "expected tool name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *endpoint == "" {
		return errors.New("endpoint is required")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "image-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: *endpoint, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		return err
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != *toolName {
		return fmt.Errorf("unexpected tools: %+v", listed.Tools)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: *toolName})
	if err != nil {
		return err
	}
	if len(result.Content) != 1 || result.Content[0].(*mcp.TextContent).Text != "image-ok" {
		return fmt.Errorf("unexpected tool result: %+v", result.Content)
	}
	return nil
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
