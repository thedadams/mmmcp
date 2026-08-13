package mmmcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/storage"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestHealthAndReadinessEndpoints(t *testing.T) {
	composite := newProbeComposite(t)

	healthRecorder, health := requestProbe(t, composite, http.MethodGet, "/healthz")
	if healthRecorder.Code != http.StatusOK || health.Status != "ok" || health.Version != implementationVersion || health.UptimeSeconds < 0 {
		t.Fatalf("health response = %d, %+v", healthRecorder.Code, health)
	}
	if health.Checks != nil {
		t.Fatalf("health checks = %+v, want none", health.Checks)
	}

	readyRecorder, ready := requestProbe(t, composite, http.MethodGet, "/readyz")
	if readyRecorder.Code != http.StatusOK || ready.Status != "ok" {
		t.Fatalf("readiness response = %d, %+v", readyRecorder.Code, ready)
	}
	for _, name := range []string{"lifecycle", "catalog", "storage"} {
		if ready.Checks[name].Status != "ok" {
			t.Fatalf("readiness check %q = %+v", name, ready.Checks[name])
		}
	}

	headRecorder, _ := requestProbe(t, composite, http.MethodHead, "/healthz")
	if headRecorder.Code != http.StatusOK || headRecorder.Body.Len() != 0 {
		t.Fatalf("HEAD /healthz = %d, body %q", headRecorder.Code, headRecorder.Body.String())
	}

	methodRecorder, methodResponse := requestProbe(t, composite, http.MethodPost, "/healthz")
	if methodRecorder.Code != http.StatusMethodNotAllowed || methodRecorder.Header().Get("Allow") != "GET, HEAD" || methodResponse.Status != "method_not_allowed" {
		t.Fatalf("POST /healthz = %d, Allow %q, response %+v", methodRecorder.Code, methodRecorder.Header().Get("Allow"), methodResponse)
	}

	if err := composite.Close(); err != nil {
		t.Fatal(err)
	}
	closedHealthRecorder, closedHealth := requestProbe(t, composite, http.MethodGet, "/healthz")
	if closedHealthRecorder.Code != http.StatusServiceUnavailable || closedHealth.Status != "unavailable" {
		t.Fatalf("closed health response = %d, %+v", closedHealthRecorder.Code, closedHealth)
	}
	closedReadyRecorder, closedReady := requestProbe(t, composite, http.MethodGet, "/readyz")
	if closedReadyRecorder.Code != http.StatusServiceUnavailable || closedReady.Checks["lifecycle"].Reason != "server_closed" {
		t.Fatalf("closed readiness response = %d, %+v", closedReadyRecorder.Code, closedReady)
	}
}

func TestReadinessReportsDegradedCatalogWithoutFailingProbe(t *testing.T) {
	composite := newProbeComposite(t)
	defer composite.Close()
	composite.catalogDegraded.Store(true)

	recorder, response := requestProbe(t, composite, http.MethodGet, "/readyz")
	if recorder.Code != http.StatusOK || response.Status != "degraded" {
		t.Fatalf("readiness response = %d, %+v", recorder.Code, response)
	}
	if check := response.Checks["catalog"]; check.Status != "degraded" || check.Reason != "refresh_failed" {
		t.Fatalf("catalog check = %+v", check)
	}
}

func TestStorageFailureAffectsReadinessNotHealth(t *testing.T) {
	composite := newProbeComposite(t)
	defer composite.Close()
	store := composite.defaultStore.(*storage.SQLStore)
	if err := store.DB().Close(); err != nil {
		t.Fatal(err)
	}

	healthRecorder, health := requestProbe(t, composite, http.MethodGet, "/healthz")
	if healthRecorder.Code != http.StatusOK || health.Status != "ok" {
		t.Fatalf("health response = %d, %+v", healthRecorder.Code, health)
	}
	readyRecorder, ready := requestProbe(t, composite, http.MethodGet, "/readyz")
	if readyRecorder.Code != http.StatusServiceUnavailable || ready.Status != "unavailable" {
		t.Fatalf("readiness response = %d, %+v", readyRecorder.Code, ready)
	}
	if check := ready.Checks["storage"]; check.Status != "error" || check.Reason != "storage_unavailable" {
		t.Fatalf("storage check = %+v", check)
	}
}

func newProbeComposite(t *testing.T) *Composite {
	t.Helper()
	fixture := testserver.New(t, testserver.Options{})
	composite, err := New(t.Context(), &config.Config{
		Servers: []config.Server{{Name: "fixture", URL: fixture.URL}},
	}, Options{DSN: t.TempDir() + "/health.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = composite.Close() })
	return composite
}

func requestProbe(t *testing.T, composite *Composite, method, path string) (*httptest.ResponseRecorder, probeResponse) {
	t.Helper()
	request := httptest.NewRequest(method, "http://example.invalid"+path, nil)
	recorder := httptest.NewRecorder()
	composite.HTTPHandler().ServeHTTP(recorder, request)
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%s %s Cache-Control = %q", method, path, recorder.Header().Get("Cache-Control"))
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("%s %s Content-Type = %q", method, path, recorder.Header().Get("Content-Type"))
	}
	var response probeResponse
	if method != http.MethodHead {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode %s %s response: %v", method, path, err)
		}
	}
	return recorder, response
}
