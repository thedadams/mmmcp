package config

import (
	"fmt"
	"strings"
)

func interpolate(value, path string, lookup func(string) (string, bool)) (string, error) {
	var result strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '$' {
			result.WriteByte(value[i])
			i++
			continue
		}
		if i+1 < len(value) && value[i+1] == '$' {
			result.WriteByte('$')
			i += 2
			continue
		}
		if i+1 >= len(value) || value[i+1] != '{' {
			result.WriteByte('$')
			i++
			continue
		}

		end := strings.IndexByte(value[i+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("%s: unterminated environment reference", path)
		}
		end += i + 2
		name := value[i+2 : end]
		if name == "" {
			return "", fmt.Errorf("%s: empty environment variable name", path)
		}
		replacement, _ := lookup(name)
		result.WriteString(replacement)
		i = end + 1
	}
	return result.String(), nil
}
