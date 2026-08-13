package mmmcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/mmmcp/config"
)

func TestConfigBridgeReplacesClientMarkerAndCleansMapping(t *testing.T) {
	requestConfig := &config.Config{Servers: []config.Server{{Name: "request"}}}
	composite := &Composite{defaultConfig: &config.Config{Servers: []config.Server{{Name: "default"}}}}
	var token string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get(privateConfigHeader)
		if token == "client-controlled" || token == "" {
			t.Fatalf("private marker was not replaced: %q", token)
		}
		selection, ok := composite.requestConfigs.get(token)
		if !ok || selection.config != requestConfig || selection.useDefault {
			t.Fatalf("selection = %+v, %v", selection, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil)
	req.Header.Set(privateConfigHeader, "client-controlled")
	req = req.WithContext(ContextWithConfig(t.Context(), requestConfig))
	recorder := httptest.NewRecorder()
	composite.configBridge(next).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if _, ok := composite.requestConfigs.get(token); ok {
		t.Fatal("request configuration mapping survived POST dispatch")
	}
}

func TestConfigBridgeRecordsExplicitDefaultForPost(t *testing.T) {
	composite := &Composite{defaultConfig: &config.Config{Servers: []config.Server{{Name: "default"}}}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selection, ok := composite.requestConfigs.get(r.Header.Get(privateConfigHeader))
		if !ok || !selection.useDefault || selection.config != nil {
			t.Fatalf("default selection = %+v, %v", selection, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	composite.configBridge(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestConfigBridgeCarriesExplicitDSN(t *testing.T) {
	composite := &Composite{defaultConfig: &config.Config{Servers: []config.Server{{Name: "default"}}}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selection, ok := composite.requestConfigs.get(r.Header.Get(privateConfigHeader))
		if !ok || !selection.dsnSet || selection.dsn != "" {
			t.Fatalf("DSN selection = %+v, %v, want explicit empty", selection, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil)
	req = req.WithContext(ContextWithDSN(req.Context(), ""))
	recorder := httptest.NewRecorder()
	composite.configBridge(next).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}
