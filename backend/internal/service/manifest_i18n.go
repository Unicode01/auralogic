package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ManifestLocalizedText struct {
	raw   interface{}
	value string
}

func (t ManifestLocalizedText) String() string {
	return strings.TrimSpace(t.value)
}

func (t *ManifestLocalizedText) UnmarshalJSON(data []byte) error {
	if t == nil {
		return nil
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		t.raw = nil
		t.value = ""
		return nil
	}

	var decoded interface{}
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return err
	}
	if _, ok := decoded.([]interface{}); ok {
		return fmt.Errorf("must be a string or localized object")
	}

	t.raw = cloneManifestLocalizedValue(decoded)
	t.value = ResolveManifestLocalizedTextValue(decoded)
	return nil
}

func (t ManifestLocalizedText) MarshalJSON() ([]byte, error) {
	if t.raw != nil {
		return json.Marshal(t.raw)
	}
	return json.Marshal(t.String())
}

func ResolveManifestLocalizedTextValue(value interface{}) string {
	if direct := manifestLocalizedScalarString(value); direct != "" {
		return direct
	}

	mapped, ok := value.(map[string]interface{})
	if !ok || len(mapped) == 0 {
		return ""
	}

	values := make(map[string]string, len(mapped))
	order := make([]string, 0, len(mapped))
	for key, item := range mapped {
		normalizedKey := normalizeManifestLocalizedKey(key)
		if normalizedKey == "" {
			continue
		}
		text := manifestLocalizedScalarString(item)
		if text == "" {
			continue
		}
		if _, exists := values[normalizedKey]; exists {
			continue
		}
		values[normalizedKey] = text
		order = append(order, normalizedKey)
	}
	if len(order) == 0 {
		return ""
	}

	for _, key := range []string{
		"default",
		"fallback",
		"value",
		"text",
		"title",
		"label",
		"en-us",
		"en",
		"zh-cn",
		"zh",
	} {
		if text := values[key]; text != "" {
			return text
		}
	}

	return values[order[0]]
}

func cloneManifestLocalizedValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			cloned[key] = cloneManifestLocalizedValue(item)
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			cloned = append(cloned, cloneManifestLocalizedValue(item))
		}
		return cloned
	default:
		return typed
	}
}

func manifestLocalizedScalarString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(typed), 'f', -1, 32))
	case int:
		return strconv.Itoa(typed)
	case int8, int16, int32, int64:
		return strings.TrimSpace(fmt.Sprintf("%d", typed))
	case uint, uint8, uint16, uint32, uint64:
		return strings.TrimSpace(fmt.Sprintf("%d", typed))
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func normalizeManifestLocalizedKey(key string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", "-"))
}
