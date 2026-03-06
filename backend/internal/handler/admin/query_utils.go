package admin

import "strings"

// parseOptionalBoolQuery parses common boolean query values.
// Empty string means "not specified".
func parseOptionalBoolQuery(raw string) (*bool, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return nil, true
	}

	switch s {
	case "1", "true", "yes", "y", "on":
		v := true
		return &v, true
	case "0", "false", "no", "n", "off":
		v := false
		return &v, true
	default:
		return nil, false
	}
}
