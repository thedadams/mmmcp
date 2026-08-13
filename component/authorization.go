package component

import (
	"fmt"
	"net/http"
)

// AuthorizationError reports an authorization challenge returned by an HTTP
// component. The challenge fields are the Bearer parameters used by MCP
// authorization flows.
type AuthorizationError struct {
	StatusCode       int
	ResourceMetadata string
	Scope            string
	Scopes           []string
	ErrorCode        string
	ErrorDescription string
}

func (e *AuthorizationError) Error() string {
	status := http.StatusText(e.StatusCode)
	if status == "" {
		status = fmt.Sprintf("status code %d", e.StatusCode)
	} else {
		status = fmt.Sprintf("%d %s", e.StatusCode, status)
	}
	if e.ErrorCode != "" && e.ErrorDescription != "" {
		return fmt.Sprintf("component authorization failed: %s: %s (%s)", status, e.ErrorCode, e.ErrorDescription)
	}
	if e.ErrorCode != "" {
		return fmt.Sprintf("component authorization failed: %s: %s", status, e.ErrorCode)
	}
	return fmt.Sprintf("component authorization failed: %s", status)
}
