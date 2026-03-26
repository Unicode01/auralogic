package catalog

import (
	"strings"
	"unicode"
)

func matchesCatalogSearch(fields []string, query string) bool {
	queryTokens := normalizeSearchTokens(query)
	if len(queryTokens) == 0 {
		return true
	}

	searchTokens := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		searchTokens = append(searchTokens, normalizeSearchTokens(field)...)
	}
	if len(searchTokens) == 0 {
		return false
	}

	for _, queryToken := range queryTokens {
		matched := false
		for _, token := range searchTokens {
			if token == queryToken {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func normalizeSearchTokens(value string) []string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return []string{}
	}

	splitter := func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}
	parts := strings.FieldsFunc(trimmed, splitter)
	return dedupeStringValues(parts...)
}
