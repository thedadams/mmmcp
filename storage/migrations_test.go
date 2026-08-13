package storage

import (
	"testing"
)

func TestSQLiteMigrationsRunBeforeOpenReturns(t *testing.T) {
	store := openTestStore(t, Options{})
	for _, table := range []string{"goose_db_version", "mmmcp_event_streams", "mmmcp_event_log"} {
		var count int
		if err := store.DB().QueryRowContext(t.Context(), `
			SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q count = %d, want 1", table, count)
		}
	}
	var version int
	if err := store.DB().QueryRowContext(t.Context(), `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("migration version = %d, want 2", version)
	}
}
