//go:build integration

package storage

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEventStoreIntegrationContract(t *testing.T) {
	tests := []struct {
		name    string
		dsn     func(*testing.T) string
		dialect Dialect
	}{
		{name: "SQLite", dsn: func(t *testing.T) string { return t.TempDir() + "/integration.db" }, dialect: DialectSQLite},
		{name: "PostgreSQL", dsn: integrationDSN("MMMCP_TEST_POSTGRES_DSN"), dialect: DialectPostgres},
		{name: "MySQL", dsn: integrationDSN("MMMCP_TEST_MYSQL_DSN"), dialect: DialectMySQL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := test.dsn(t)
			store, err := Open(t.Context(), dsn, Options{MaxEventBytes: 4, MaxOpenConns: 4, MaxIdleConns: 2})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			if store.Dialect() != test.dialect {
				t.Fatalf("dialect = %q, want %q", store.Dialect(), test.dialect)
			}

			ctx := t.Context()
			sessionID := fmt.Sprintf("integration-%d", time.Now().UnixNano())
			if err := store.Open(ctx, sessionID, "stream"); err != nil {
				t.Fatal(err)
			}
			for _, value := range [][]byte{nil, {}, []byte("aaaa"), []byte("bbbb")} {
				if err := store.Append(ctx, sessionID, "stream", value); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := collectEvents(store.After(ctx, sessionID, "stream", -1)); !errors.Is(err, mcp.ErrEventsPurged) {
				t.Fatalf("purged replay error = %v, want ErrEventsPurged", err)
			}
			values, err := collectEvents(store.After(ctx, sessionID, "stream", 2))
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != 1 || string(values[0]) != "bbbb" {
				t.Fatalf("retained values = %q, want [bbbb]", values)
			}
			if err := store.SessionClosed(ctx, sessionID); err != nil {
				t.Fatal(err)
			}
			if err := store.SessionClosed(ctx, sessionID); err != nil {
				t.Fatalf("idempotent session cleanup: %v", err)
			}

			concurrentStore, err := Open(ctx, dsn, Options{MaxEventBytes: 1 << 20, MaxOpenConns: 4, MaxIdleConns: 2})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = concurrentStore.Close() })
			concurrentSession := sessionID + "-concurrent"
			if err := concurrentStore.Open(ctx, concurrentSession, "stream"); err != nil {
				t.Fatal(err)
			}
			const eventCount = 24
			var wg sync.WaitGroup
			errors := make(chan error, eventCount)
			for i := range eventCount {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errors <- concurrentStore.Append(ctx, concurrentSession, "stream", []byte(fmt.Sprintf("event-%d", i)))
				}()
			}
			wg.Wait()
			close(errors)
			for err := range errors {
				if err != nil {
					t.Fatal(err)
				}
			}
			values, err = collectEvents(concurrentStore.After(ctx, concurrentSession, "stream", -1))
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != eventCount {
				t.Fatalf("concurrent event count = %d, want %d", len(values), eventCount)
			}
			if err := concurrentStore.SessionClosed(ctx, concurrentSession); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func integrationDSN(environment string) func(*testing.T) string {
	return func(t *testing.T) string {
		dsn := os.Getenv(environment)
		if dsn == "" {
			t.Skipf("%s is not set", environment)
		}
		return dsn
	}
}
