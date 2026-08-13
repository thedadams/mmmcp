package mmmcp

import (
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type httpActivityRegistry struct {
	mu       sync.Mutex
	active   map[string]int
	sessions map[string]*mcp.ServerSession
}

func (r *httpActivityRegistry) begin(sessionID string) int {
	if sessionID == "" {
		return 0
	}
	r.mu.Lock()
	if r.active == nil {
		r.active = make(map[string]int)
	}
	r.active[sessionID]++
	active := r.active[sessionID]
	r.mu.Unlock()
	return active
}

func (r *httpActivityRegistry) end(sessionID string) int {
	if sessionID == "" {
		return 0
	}
	r.mu.Lock()
	if r.active[sessionID] > 1 {
		r.active[sessionID]--
	} else {
		delete(r.active, sessionID)
	}
	active := r.active[sessionID]
	r.mu.Unlock()
	return active
}

func (r *httpActivityRegistry) bind(session *mcp.ServerSession) int {
	if session == nil || session.ID() == "" {
		return 0
	}
	r.mu.Lock()
	if r.sessions == nil {
		r.sessions = make(map[string]*mcp.ServerSession)
	}
	r.sessions[session.ID()] = session
	active := r.active[session.ID()]
	r.mu.Unlock()
	return active
}

func (r *httpActivityRegistry) session(sessionID string) *mcp.ServerSession {
	r.mu.Lock()
	session := r.sessions[sessionID]
	r.mu.Unlock()
	return session
}

func (c *Composite) trackHTTPActivity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIsStateless(r) {
			next.ServeHTTP(w, r)
			return
		}
		sessionID := r.Header.Get("Mcp-Session-Id")
		active := c.activity.begin(sessionID)
		if session := c.activity.session(sessionID); session != nil {
			c.pool.SetFrontendActivity(session, active)
		}
		defer func() {
			active := c.activity.end(sessionID)
			if session := c.activity.session(sessionID); session != nil {
				c.pool.SetFrontendActivity(session, active)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (c *Composite) bindFrontendActivity(session *mcp.ServerSession) int {
	active := c.activity.bind(session)
	c.pool.SetFrontendActivity(session, active)
	return active
}
