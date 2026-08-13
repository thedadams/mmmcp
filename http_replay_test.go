package mmmcp_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp"
	"github.com/obot-platform/mmmcp/config"
	"github.com/obot-platform/mmmcp/testserver"
)

func TestStatefulSSEReplayPersistsInSQLiteAndCleansUp(t *testing.T) {
	var progress atomic.Int32
	fixture := testserver.New(t, testserver.Options{
		Stateful: true,
		Tools: []testserver.Tool{{
			Definition: &mcp.Tool{Name: "progress", InputSchema: map[string]any{"type": "object"}},
			Handler: func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				value := progress.Add(1)
				if err := req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
					ProgressToken: "token",
					Progress:      float64(value),
					Message:       "progress-" + strconv.Itoa(int(value)),
				}); err != nil {
					return nil, err
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			},
		}},
	})
	dsn := t.TempDir() + "/replay.db"
	composite, err := mmmcp.New(t.Context(), &config.Config{
		Servers: []config.Server{{Name: "fixture", URL: fixture.URL}},
	}, mmmcp.Options{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()
	frontend := httptest.NewServer(composite.HTTPHandler())
	defer frontend.Close()

	currentClient := mcp.NewClient(&mcp.Implementation{Name: "current-test", Version: "1.0.0"}, nil)
	current, err := currentClient.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             frontend.URL,
		HTTPClient:           frontend.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := current.ListTools(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if err := current.Close(); err != nil {
		t.Fatal(err)
	}
	if got := countEventStreams(t, dsn); got != 0 {
		t.Fatalf("stateless current-protocol stream count = %d, want 0", got)
	}

	legacy := connectLegacy(t, frontend, frontend.Client())
	sessionID := legacy.ID()
	response, cancel := openSSE(t, frontend.Client(), frontend.URL, sessionID, "")
	if _, err := legacy.CallTool(t.Context(), &mcp.CallToolParams{Name: "progress"}); err != nil {
		cancel()
		response.Body.Close()
		t.Fatal(err)
	}
	firstID, firstData := readSSEMessage(t, response.Body)
	if firstID == "" || !strings.Contains(string(firstData), "progress-1") {
		cancel()
		response.Body.Close()
		t.Fatalf("first SSE event id=%q data=%s", firstID, firstData)
	}
	cancel()
	response.Body.Close()

	if _, err := legacy.CallTool(t.Context(), &mcp.CallToolParams{Name: "progress"}); err != nil {
		t.Fatal(err)
	}
	replayed, cancelReplay := openSSE(t, frontend.Client(), frontend.URL, sessionID, firstID)
	secondID, secondData := readSSEMessage(t, replayed.Body)
	if secondID == firstID || !strings.Contains(string(secondData), "progress-2") {
		cancelReplay()
		replayed.Body.Close()
		t.Fatalf("replayed SSE event id=%q data=%s", secondID, secondData)
	}
	var message struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(secondData, &message); err != nil || message.Method != "notifications/progress" {
		cancelReplay()
		replayed.Body.Close()
		t.Fatalf("replayed JSON-RPC message = %s, error = %v", secondData, err)
	}
	cancelReplay()
	replayed.Body.Close()

	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return countEventStreams(t, dsn) == 0 })
}

func TestStatefulReplayStoreRemainsBoundToOpeningDSN(t *testing.T) {
	fixture := namedToolFixture(t, "ok")
	firstDSN := t.TempDir() + "/first.db"
	secondDSN := t.TempDir() + "/second.db"
	firstConfig := &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}
	secondConfig := &config.Config{Servers: []config.Server{{Name: "fixture", URL: fixture.URL}}}
	composite, err := mmmcp.New(t.Context(), firstConfig, mmmcp.Options{DSN: firstDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer composite.Close()

	var selected atomic.Pointer[config.Config]
	selected.Store(firstConfig)
	var selectedDSN atomic.Pointer[string]
	selectedDSN.Store(&firstDSN)
	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg := selected.Load(); cfg != nil {
			r = r.WithContext(mmmcp.ContextWithConfig(r.Context(), cfg))
		}
		if dsn := selectedDSN.Load(); dsn != nil {
			r = r.WithContext(mmmcp.ContextWithDSN(r.Context(), *dsn))
		}
		composite.HTTPHandler().ServeHTTP(w, r)
	}))
	defer frontend.Close()
	legacy := connectLegacy(t, frontend, frontend.Client())
	defer legacy.Close()

	selected.Store(secondConfig)
	selectedDSN.Store(&secondDSN)
	if _, err := legacy.ListTools(t.Context(), &mcp.ListToolsParams{}); err != nil {
		t.Fatal(err)
	}
	if got := countEventStreams(t, firstDSN); got < 2 {
		t.Fatalf("opening store stream count = %d, want at least 2", got)
	}
	if _, err := os.Stat(secondDSN); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("later request unexpectedly opened second store: %v", err)
	}
}

func openSSE(t *testing.T, client *http.Client, endpoint, sessionID, lastEventID string) (*http.Response, context.CancelFunc) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithCancel(t.Context())
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		request.Header.Set("Accept", "text/event-stream")
		request.Header.Set("Mcp-Session-Id", sessionID)
		request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		if lastEventID != "" {
			request.Header.Set("Last-Event-ID", lastEventID)
		}
		response, err := client.Do(request)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if response.StatusCode == http.StatusOK {
			return response, cancel
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		cancel()
		if response.StatusCode != http.StatusConflict || time.Now().After(deadline) {
			t.Fatalf("open SSE status = %d, body = %s", response.StatusCode, body)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readSSEMessage(t *testing.T, body io.Reader) (string, []byte) {
	t.Helper()
	reader := bufio.NewReader(body)
	var id string
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(data) > 0 {
				return id, []byte(strings.Join(data, "\n"))
			}
			id = ""
			continue
		}
		if value, ok := strings.CutPrefix(line, "id:"); ok {
			id = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimSpace(value))
		}
	}
}

func countEventStreams(t *testing.T, dsn string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mmmcp_event_streams`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
