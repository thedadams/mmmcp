package storage

import (
	"testing"
)

func TestRegistryClosesEveryOwnedPool(t *testing.T) {
	registry := NewRegistry(Options{})
	first, err := registry.Get(t.Context(), t.TempDir()+"/first.db")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Get(t.Context(), t.TempDir()+"/second.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	for _, store := range []Store{first, second} {
		err := store.(*SQLStore).DB().PingContext(t.Context())
		if err == nil {
			t.Fatal("closed pool ping unexpectedly succeeded")
		}
	}
	if _, err := registry.Get(t.Context(), t.TempDir()+"/third.db"); err == nil {
		t.Fatal("closed registry accepted a new store")
	}
}

func TestSQLitePoolRemainsSingleConnection(t *testing.T) {
	store := openTestStore(t, Options{MaxOpenConns: 20, MaxIdleConns: 10})
	if got := store.DB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("SQLite max open connections = %d, want 1", got)
	}
}
