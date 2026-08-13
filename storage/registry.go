package storage

import (
	"context"
	"errors"
	"sync"
)

type registryEntry struct {
	ready chan struct{}
	store Store
	err   error
}

// Registry lazily opens and deduplicates stores by a non-reversible DSN
// digest. It owns every store it returns.
type Registry struct {
	options Options

	mu      sync.Mutex
	entries map[string]*registryEntry
	closed  bool
}

// NewRegistry creates a storage registry.
func NewRegistry(options Options) *Registry {
	return &Registry{options: options.normalized(), entries: make(map[string]*registryEntry)}
}

// Get returns the one opened store for dsn.
func (r *Registry) Get(ctx context.Context, dsn string) (Store, error) {
	if r == nil {
		return nil, errors.New("storage: nil registry")
	}
	classified, err := classifyDSN(dsn, r.options)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, errors.New("storage: registry is closed")
	}
	if entry := r.entries[classified.key]; entry != nil {
		r.mu.Unlock()
		select {
		case <-entry.ready:
			return entry.store, entry.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	entry := &registryEntry{ready: make(chan struct{})}
	r.entries[classified.key] = entry
	r.mu.Unlock()

	entry.store, entry.err = Open(ctx, dsn, r.options)
	r.mu.Lock()
	if entry.err != nil {
		delete(r.entries, classified.key)
	}
	close(entry.ready)
	r.mu.Unlock()
	return entry.store, entry.err
}

// Close closes all stores owned by the registry.
func (r *Registry) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	entries := make([]*registryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.Unlock()

	var result error
	for _, entry := range entries {
		<-entry.ready
		if entry.store != nil {
			result = errors.Join(result, entry.store.Close())
		}
	}
	return result
}
