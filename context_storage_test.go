package mmmcp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/mmmcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/storage"
)

func TestEmptyContextDSNSelectsSQLiteInsteadOfDefaultDSN(t *testing.T) {
	fixture := namedToolFixture(t, "ok")
	defaultDSN := t.TempDir() + "/default.db"
	dataDirectory := t.TempDir()
	defaultConfig := &config.Config{
		Servers: []config.Server{{Name: "fixture", URL: fixture.URL}},
	}
	requestConfig := &config.Config{
		Servers: []config.Server{{Name: "fixture", URL: fixture.URL}},
	}
	composite, err := mmmcp.New(t.Context(), defaultConfig, mmmcp.Options{
		DSN: defaultDSN, Storage: storage.Options{DataDirectory: dataDirectory},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()

	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(mmmcp.ContextWithDSN(mmmcp.ContextWithConfig(r.Context(), requestConfig), ""))
		composite.HTTPHandler().ServeHTTP(w, r)
	}))
	defer frontend.Close()
	legacy := connectLegacy(t, frontend, frontend.Client())
	defer legacy.Close()

	contextDSN := filepath.Join(dataDirectory, "mmmcp.db")
	if got := countEventStreams(t, contextDSN); got == 0 {
		t.Fatal("empty context DSN did not open the context configuration's SQLite store")
	}
	if got := countEventStreams(t, defaultDSN); got != 0 {
		t.Fatalf("context session wrote %d streams to the default DSN", got)
	}
}

func TestOptionsDSNSelectsDefaultStorage(t *testing.T) {
	fixture := namedToolFixture(t, "ok")
	overrideDSN := t.TempDir() + "/override.db"
	cfg := &config.Config{
		Servers: []config.Server{{Name: "fixture", URL: fixture.URL}},
	}
	composite, err := mmmcp.New(t.Context(), cfg, mmmcp.Options{DSN: overrideDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	if _, err := os.Stat(overrideDSN); err != nil {
		t.Fatalf("programmatic default DSN was not opened: %v", err)
	}
}
