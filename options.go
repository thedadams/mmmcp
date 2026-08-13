package mmmcp

import (
	"log/slog"
	nethttp "net/http"
	"time"

	"github.com/obot-platform/mmmcp/component/http"
	"github.com/obot-platform/mmmcp/storage"
)

// Options configures the composite runtime.
type Options struct {
	Logger     *slog.Logger
	HTTPClient *nethttp.Client
	// OAuth supplies a component-specific OAuth handler for downstream HTTP clients.
	OAuth http.OAuthHandlerProvider
	// DSN selects the default event storage. An empty DSN selects SQLite.
	DSN string
	// LookupEnv supplies the allow-listed baseline for command components.
	LookupEnv func(string) (string, bool)
	// CommandTerminateDuration controls each SDK command shutdown stage.
	CommandTerminateDuration time.Duration
	// PageSize limits each composite feature-list page. Zero returns a complete family.
	PageSize int
	// Storage controls SQL connection pools, data location, and event retention.
	Storage storage.Options
}
