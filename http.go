package mmmcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/component"
)

func (c *Composite) newHTTPHandler(opts Options) http.Handler {
	stateless := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newFrontendServer(c, opts) }, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		Logger:                       opts.Logger,
		PropagateRequestCancellation: true,
	})
	stateful := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newFrontendServer(c, opts) }, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Logger:       opts.Logger,
		EventStore:   c.events,
	})
	dispatch := &httpDispatcher{stateless: stateless, stateful: stateful}
	bridged := c.configBridge(c.trackHTTPActivity(dispatch))
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.closed.Load() {
			http.Error(w, "composite server is closed", http.StatusServiceUnavailable)
			return
		}
		forwardedHeaders := r.Header.Clone()
		forwardedHeaders.Del(authorizationErrorCaptureHeader)
		r = r.WithContext(component.ContextWithRequestHeaders(r.Context(), forwardedHeaders))
		if r.Method == http.MethodPost && !isSubscriptionsListenRequest(r) {
			c.serveBufferedAuthorizationResponse(w, r, bridged)
			return
		}
		bridged.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			c.serveHealth(w, r)
		case "/readyz":
			c.serveReady(w, r)
		default:
			mcpHandler.ServeHTTP(w, r)
		}
	})
}
