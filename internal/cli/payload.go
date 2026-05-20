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

func payloadBool(response protocol.Response, key string) bool {
	value, ok := response.Payload[key]
	if !ok || value == nil {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}
