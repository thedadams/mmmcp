package mmmcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPDispatcherMatrix(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		headers map[string]string
		body    string
		want    string
	}{
		{name: "current header", method: http.MethodPost, headers: map[string]string{"Mcp-Protocol-Version": currentProtocolVersion}, body: `{}`, want: "stateless"},
		{name: "current header ignores legacy session header", method: http.MethodPost, headers: map[string]string{"Mcp-Protocol-Version": currentProtocolVersion, "Mcp-Session-Id": "stale"}, body: `{}`, want: "stateless"},
		{name: "discover shape", method: http.MethodPost, body: `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{}}`, want: "stateless"},
		{name: "current metadata", method: http.MethodPost, body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`, want: "stateless"},
		{name: "initialize", method: http.MethodPost, body: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, want: "stateful"},
		{name: "legacy session post", method: http.MethodPost, headers: map[string]string{"Mcp-Session-Id": "session"}, body: `{}`, want: "stateful"},
		{name: "get", method: http.MethodGet, want: "stateful"},
		{name: "delete", method: http.MethodDelete, want: "stateful"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dispatcher := &httpDispatcher{
				stateless: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "stateless") }),
				stateful:  http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "stateful") }),
			}
			req := httptest.NewRequest(test.method, "http://example.invalid/mcp", strings.NewReader(test.body))
			for name, value := range test.headers {
				req.Header.Set(name, value)
			}
			recorder := httptest.NewRecorder()
			dispatcher.ServeHTTP(recorder, req)
			if got := recorder.Body.String(); got != test.want {
				t.Fatalf("route = %q, want %q", got, test.want)
			}
		})
	}
}
