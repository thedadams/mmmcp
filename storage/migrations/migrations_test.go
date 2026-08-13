package migrations

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

func TestSQLiteRenameMigrationUpgradesExistingDatabase(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	})

	dialectFiles, err := fs.Sub(files, string(SQLite))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, dialectFiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	assertSQLiteTable(t, db, "mmcp_event_streams", true)
	assertSQLiteTable(t, db, "mmcp_event_log", true)
	if _, err := db.Exec(`
		INSERT INTO mmcp_event_streams
			(session_id, stream_id, first_retained_index, next_index, expires_at, updated_at)
		VALUES ('session', 'stream', 0, 1, 100, 10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO mmcp_event_log
			(session_id, stream_id, event_index, payload, payload_is_null, payload_size, created_at)
		VALUES ('session', 'stream', 0, 'event', 0, 5, 10)`); err != nil {
		t.Fatal(err)
	}

	if err := Up(ctx, db, SQLite); err != nil {
		t.Fatal(err)
	}
	assertSQLiteTable(t, db, "mmcp_event_streams", false)
	assertSQLiteTable(t, db, "mmcp_event_log", false)
	assertSQLiteTable(t, db, "mmmcp_event_streams", true)
	assertSQLiteTable(t, db, "mmmcp_event_log", true)
	var payload string
	if err := db.QueryRow(`
		SELECT payload FROM mmmcp_event_log
		WHERE session_id = 'session' AND stream_id = 'stream' AND event_index = 0`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload != "event" {
		t.Fatalf("migrated payload = %q, want event", payload)
	}
}

func assertSQLiteTable(t *testing.T, db *sql.DB, name string, want bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if got := count == 1; got != want {
		t.Fatalf("table %q exists = %t, want %t", name, got, want)
	}
}
