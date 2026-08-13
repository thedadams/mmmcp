package stdio

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestEnvironmentAllowsBaselineAndAppendsConfiguredValues(t *testing.T) {
	host := map[string]string{
		"PATH":          "/host/bin",
		"PWD":           "/host/work",
		"LANG":          "en_US.UTF-8",
		"MMMCP_SECRET":  "must-not-leak",
		"EXPLICIT_ONLY": "host-value",
	}
	env := Environment(func(name string) (string, bool) {
		value, ok := host[name]
		return value, ok
	}, map[string]string{
		"PATH":          "/component/bin",
		"EXPLICIT_ONLY": "configured",
	})

	if env == nil {
		t.Fatal("Environment returned nil")
	}
	if slices.Contains(env, "MMMCP_SECRET=must-not-leak") {
		t.Fatalf("environment leaked host secret: %v", env)
	}
	if got := lastValue(env, "PATH"); got != "/component/bin" {
		t.Fatalf("PATH = %q, want configured value", got)
	}
	if got := lastValue(env, "PWD"); got != "/host/work" {
		t.Fatalf("PWD = %q, want host baseline", got)
	}
	if got := lastValue(env, "EXPLICIT_ONLY"); got != "configured" {
		t.Fatalf("EXPLICIT_ONLY = %q", got)
	}
	if runtime.GOOS != "windows" && lastValue(env, "SystemRoot") != "" {
		t.Fatalf("unexpected Windows launch variable: %v", env)
	}
}

func TestEnvironmentIsNonNilWhenEmpty(t *testing.T) {
	env := Environment(func(string) (string, bool) { return "", false }, nil)
	if env == nil || len(env) != 0 {
		t.Fatalf("environment = %#v, want non-nil empty slice", env)
	}
}

func lastValue(env []string, name string) string {
	prefix := name + "="
	var result string
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			result = value
		}
	}
	return result
}
