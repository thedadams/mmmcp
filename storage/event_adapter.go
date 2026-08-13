package storage

import (
	"context"
	"errors"
	"iter"
	"sync"
)

// DSNSelector selects the complete effective configuration's DSN for the
// request that creates a stateful MCP session.
type DSNSelector func(context.Context) string

// EventAdapter binds each MCP session to the store selected by its first
// stream. Later requests cannot move that session to another database.
type EventAdapter struct {
	registry  *Registry
	selectDSN DSNSelector

	mu       sync.Mutex
	sessions map[string]Store
}

// NewEventAdapter creates a session-affine event-store adapter.
func NewEventAdapter(registry *Registry, selectDSN DSNSelector) *EventAdapter {
	return &EventAdapter{registry: registry, selectDSN: selectDSN, sessions: make(map[string]Store)}
}

// Open binds sessionID on first use and opens streamID in that store.
func (a *EventAdapter) Open(ctx context.Context, sessionID, streamID string) error {
	a.mu.Lock()
	store := a.sessions[sessionID]
	created := false
	if store == nil {
		if a.registry == nil {
			a.mu.Unlock()
			return errors.New("storage: event adapter has no registry")
		}
		var dsn string
		if a.selectDSN != nil {
			dsn = a.selectDSN(ctx)
		}
		var err error
		store, err = a.registry.Get(ctx, dsn)
		if err != nil {
			a.mu.Unlock()
			return err
		}
		a.sessions[sessionID] = store
		created = true
	}
	a.mu.Unlock()
	if err := store.Open(ctx, sessionID, streamID); err != nil {
		if created {
			a.mu.Lock()
			if a.sessions[sessionID] == store {
				delete(a.sessions, sessionID)
			}
			a.mu.Unlock()
		}
		return err
	}
	return nil
}

// Append writes to the store permanently bound to sessionID.
func (a *EventAdapter) Append(ctx context.Context, sessionID, streamID string, data []byte) error {
	store := a.store(sessionID)
	if store == nil {
		return errors.New("storage: event session is not open")
	}
	return store.Append(ctx, sessionID, streamID, data)
}

// After replays from the store permanently bound to sessionID.
func (a *EventAdapter) After(ctx context.Context, sessionID, streamID string, index int) iter.Seq2[[]byte, error] {
	store := a.store(sessionID)
	if store == nil {
		return func(yield func([]byte, error) bool) {
			yield(nil, errors.New("storage: event session is not open"))
		}
	}
	return store.After(ctx, sessionID, streamID, index)
}

// SessionClosed removes persisted events and releases the session binding.
func (a *EventAdapter) SessionClosed(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	store := a.sessions[sessionID]
	a.mu.Unlock()
	if store == nil {
		return nil
	}
	if err := store.SessionClosed(ctx, sessionID); err != nil {
		return err
	}
	a.mu.Lock()
	if a.sessions[sessionID] == store {
		delete(a.sessions, sessionID)
	}
	a.mu.Unlock()
	return nil
}

func (a *EventAdapter) store(sessionID string) Store {
	a.mu.Lock()
	store := a.sessions[sessionID]
	a.mu.Unlock()
	return store
}
