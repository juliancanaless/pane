package cli

import (
	"fmt"

	"github.com/juliancanalez/pane/internal/protocol"
)

func payloadString(response protocol.Response, key string) string {
	value, ok := response.Payload[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Sprint(value)
	}
	return text
}

func payloadMaps(response protocol.Response, key string) []map[string]any {
	value, ok := response.Payload[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]map[string]any); ok {
			return typed
		}
		return nil
	}
	maps := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if typed, ok := item.(map[string]any); ok {
			maps = append(maps, typed)
		}
	}
	return maps
}

func mapString(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Sprint(value)
	}
	return text
}

func mapInt64(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func payloadBool(response protocol.Response, key string) bool {
	value, ok := response.Payload[key]
	if !ok || value == nil {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}
