package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSQLiteEventStoreContract(t *testing.T) {
	store := openTestStore(t, Options{})
	ctx := t.Context()
	if err := store.Open(ctx, "session", "stream"); err != nil {
		t.Fatal(err)
	}
	for _, value := range [][]byte{nil, {}, []byte("value")} {
		if err := store.Append(ctx, "session", "stream", value); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectEvents(store.After(ctx, "session", "stream", -1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != nil || got[1] == nil || len(got[1]) != 0 || string(got[2]) != "value" {
		t.Fatalf("replayed events = %#v", got)
	}
	got, err = collectEvents(store.After(ctx, "session", "stream", 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] == nil || len(got[0]) != 0 || string(got[1]) != "value" {
		t.Fatalf("events after index 0 = %#v", got)
	}

	if err := store.SessionClosed(ctx, "session"); err != nil {
		t.Fatal(err)
	}
	if err := store.SessionClosed(ctx, "session"); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	if _, err := collectEvents(store.After(ctx, "session", "stream", -1)); err == nil {
		t.Fatal("replay after session cleanup unexpectedly succeeded")
	}
}

func TestSQLiteEventStoreConcurrentAppendAllocatesEveryPosition(t *testing.T) {
	store := openTestStore(t, Options{})
	ctx := t.Context()
	if err := store.Open(ctx, "session", "stream"); err != nil {
		t.Fatal(err)
	}

	const count = 64
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := range count {
		wg.Go(func() {
			errs <- store.Append(ctx, "session", "stream", []byte(fmt.Sprintf("event-%d", i)))
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	values, err := collectEvents(store.After(ctx, "session", "stream", -1))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != count {
		t.Fatalf("event count = %d, want %d", len(values), count)
	}
	seen := make(map[string]bool, count)
	for _, value := range values {
		seen[string(value)] = true
	}
	if len(seen) != count {
		t.Fatalf("unique event count = %d, want %d", len(seen), count)
	}
}

func TestSQLiteEventStorePurgedReplayFailsBeforeYieldingData(t *testing.T) {
	store := openTestStore(t, Options{MaxEventBytes: 4})
	ctx := t.Context()
	if err := store.Open(ctx, "session", "stream"); err != nil {
		t.Fatal(err)
	}
	for _, value := range [][]byte{[]byte("aaaa"), []byte("bbbb")} {
		if err := store.Append(ctx, "session", "stream", value); err != nil {
			t.Fatal(err)
		}
	}

	var yielded [][]byte
	var replayErr error
	for value, err := range store.After(ctx, "session", "stream", -1) {
		if err != nil {
			replayErr = err
			break
		}
		yielded = append(yielded, value)
	}
	if !errors.Is(replayErr, mcp.ErrEventsPurged) {
		t.Fatalf("replay error = %v, want ErrEventsPurged", replayErr)
	}
	if len(yielded) != 0 {
		t.Fatalf("purged replay yielded partial data: %q", yielded)
	}
	values, err := collectEvents(store.After(ctx, "session", "stream", 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || string(values[0]) != "bbbb" {
		t.Fatalf("retained events = %q", values)
	}
}

func TestSQLiteEventStorePrunesAbandonedSessionsByTTL(t *testing.T) {
	var mu sync.Mutex
	now := time.Unix(1_000, 0)
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	store := openTestStore(t, Options{EventTTL: time.Minute, Now: clock})
	ctx := t.Context()
	if err := store.Open(ctx, "abandoned", "stream"); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, "abandoned", "stream", []byte("event")); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	if err := store.Open(ctx, "active", "stream"); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM mmmcp_event_streams WHERE session_id = 'abandoned'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("abandoned stream count = %d, want 0", count)
	}
}

func TestStorageRegistryDeduplicatesConcurrentSQLiteOpens(t *testing.T) {
	registry := NewRegistry(Options{})
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Fatal(err)
		}
	})
	dsn := t.TempDir() + "/events.db"
	const count = 16
	stores := make(chan Store, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			store, err := registry.Get(t.Context(), dsn)
			stores <- store
			errs <- err
		})
	}
	wg.Wait()
	close(stores)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first Store
	for store := range stores {
		if first == nil {
			first = store
		} else if store != first {
			t.Fatal("registry returned distinct stores for one DSN")
		}
	}
}

func TestEventAdapterKeepsOpeningStoreAffinity(t *testing.T) {
	registry := NewRegistry(Options{})
	t.Cleanup(func() { _ = registry.Close() })
	firstDSN := t.TempDir() + "/first.db"
	secondDSN := t.TempDir() + "/second.db"
	type key struct{}
	adapter := NewEventAdapter(registry, func(ctx context.Context) string {
		return ctx.Value(key{}).(string)
	})
	ctxFirst := context.WithValue(t.Context(), key{}, firstDSN)
	ctxSecond := context.WithValue(t.Context(), key{}, secondDSN)
	if err := adapter.Open(ctxFirst, "session", "first-stream"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Open(ctxSecond, "session", "second-stream"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Append(ctxSecond, "session", "second-stream", []byte("event")); err != nil {
		t.Fatal(err)
	}

	first, err := registry.Get(t.Context(), firstDSN)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Get(t.Context(), secondDSN)
	if err != nil {
		t.Fatal(err)
	}
	if got := streamCount(t, first.(*SQLStore), "session"); got != 2 {
		t.Fatalf("opening store stream count = %d, want 2", got)
	}
	if got := streamCount(t, second.(*SQLStore), "session"); got != 0 {
		t.Fatalf("later request store stream count = %d, want 0", got)
	}
}

func TestStorageErrorsDoNotExposeDSNSecrets(t *testing.T) {
	secret := "do-not-print-this-password"
	_, err := Open(t.Context(), "postgres://user:"+secret+"@example.invalid/db", Options{})
	if err == nil {
		t.Fatal("unsupported DSN unexpectedly opened")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposes DSN secret: %v", err)
	}
}

func openTestStore(t *testing.T, options Options) *SQLStore {
	t.Helper()
	store, err := Open(t.Context(), t.TempDir()+"/events.db", options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return store
}

func collectEvents(sequence func(func([]byte, error) bool)) ([][]byte, error) {
	var result [][]byte
	for value, err := range sequence {
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func streamCount(t *testing.T, store *SQLStore, sessionID string) int {
	t.Helper()
	var count int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM mmmcp_event_streams WHERE session_id = ?`, sessionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
