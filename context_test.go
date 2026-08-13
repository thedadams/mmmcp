package mmmcp

import (
	"testing"

	"github.com/obot-platform/mmmcp/config"
)

func TestContextConfigIsExactReplacement(t *testing.T) {
	defaultConfig := &config.Config{Servers: []config.Server{{Name: "default"}}}
	requestConfig := &config.Config{Servers: []config.Server{{Name: "request"}}}

	ctx := ContextWithConfig(t.Context(), requestConfig)
	got, ok := ConfigFromContext(ctx)
	if !ok || got != requestConfig {
		t.Fatalf("ConfigFromContext() = %p, %v, want %p, true", got, ok, requestConfig)
	}
	if selected := effectiveConfig(ctx, defaultConfig); selected != requestConfig || selected.Servers[0].Name != "request" {
		t.Fatalf("effectiveConfig() = %+v, want exact request config", selected)
	}
	if selected := effectiveConfig(t.Context(), defaultConfig); selected != defaultConfig {
		t.Fatalf("effectiveConfig(background) = %p, want %p", selected, defaultConfig)
	}
}

func TestContextDSNPresenceDistinguishesAbsentAndEmpty(t *testing.T) {
	if _, ok := DSNFromContext(t.Context()); ok {
		t.Fatal("background context unexpectedly has a DSN")
	}
	ctx := ContextWithDSN(t.Context(), "")
	if dsn, ok := DSNFromContext(ctx); !ok || dsn != "" {
		t.Fatalf("DSNFromContext() = %q, %v, want explicit empty", dsn, ok)
	}
	if got := effectiveDSN(ctx, "default.db"); got != "" {
		t.Fatalf("effectiveDSN() = %q, want empty", got)
	}
}

func TestContextWithConfigRejectsNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("ContextWithConfig did not panic for nil config")
		}
	}()
	ContextWithConfig(t.Context(), nil)
}
