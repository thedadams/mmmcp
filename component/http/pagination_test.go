package http

import (
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestPaginateTreatsMethodNotFoundAsEmptyList(t *testing.T) {
	for _, method := range []string{"tools/list", "prompts/list", "resources/list", "resources/templates/list"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			err := paginate("fixture", method, func(string) (string, error) {
				calls++
				return "", fmt.Errorf("list response: %w", &jsonrpc.Error{
					Code:    jsonrpc.CodeMethodNotFound,
					Message: "Method not found",
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("page calls = %d, want 1", calls)
			}
		})
	}
}
