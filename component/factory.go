package component

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/config"
)

// RoutingFactory selects a transport-specific factory for each component.
type RoutingFactory struct {
	http  Factory
	stdio Factory
}

// NewRoutingFactory creates a factory for HTTP and command components.
func NewRoutingFactory(httpFactory, stdioFactory Factory) *RoutingFactory {
	return &RoutingFactory{http: httpFactory, stdio: stdioFactory}
}

func (f *RoutingFactory) selected(server config.Server) (Factory, error) {
	if server.URL != "" && server.Command == "" && f.http != nil {
		return f.http, nil
	}
	if server.Command != "" && server.URL == "" && f.stdio != nil {
		return f.stdio, nil
	}
	return nil, fmt.Errorf("component %q has no valid transport", server.Name)
}

func (f *RoutingFactory) Discover(ctx context.Context, server config.Server) (*Features, error) {
	selected, err := f.selected(server)
	if err != nil {
		return nil, err
	}
	return selected.Discover(ctx, server)
}

func (f *RoutingFactory) CallTool(ctx context.Context, server config.Server, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	selected, err := f.selected(server)
	if err != nil {
		return nil, err
	}
	return selected.CallTool(ctx, server, params)
}

func (f *RoutingFactory) GetPrompt(ctx context.Context, server config.Server, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	selected, err := f.selected(server)
	if err != nil {
		return nil, err
	}
	return selected.GetPrompt(ctx, server, params)
}

func (f *RoutingFactory) ReadResource(ctx context.Context, server config.Server, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	selected, err := f.selected(server)
	if err != nil {
		return nil, err
	}
	return selected.ReadResource(ctx, server, params)
}

func (f *RoutingFactory) Subscribe(ctx context.Context, server config.Server, params *mcp.SubscribeParams) error {
	selected, err := f.selected(server)
	if err != nil {
		return err
	}
	return selected.Subscribe(ctx, server, params)
}

func (f *RoutingFactory) Unsubscribe(ctx context.Context, server config.Server, params *mcp.UnsubscribeParams) error {
	selected, err := f.selected(server)
	if err != nil {
		return err
	}
	return selected.Unsubscribe(ctx, server, params)
}

func (f *RoutingFactory) OpenRuntime(ctx context.Context, server config.Server, opts RuntimeOptions) (Runtime, error) {
	selected, err := f.selected(server)
	if err != nil {
		return nil, err
	}
	return selected.OpenRuntime(ctx, server, opts)
}

// Close closes both transport-specific factories.
func (f *RoutingFactory) Close() error {
	var httpErr, stdioErr error
	if f.http != nil {
		httpErr = f.http.Close()
	}
	if f.stdio != nil {
		stdioErr = f.stdio.Close()
	}
	return errors.Join(httpErr, stdioErr)
}
