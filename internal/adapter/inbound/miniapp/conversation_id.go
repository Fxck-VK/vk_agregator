package miniapp

import "strings"

const (
	defaultConversationID = "default"
	maxConversationIDLen  = 64
)

func normalizeConversationID(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultConversationID, true
	}
	if len(value) > maxConversationIDLen {
		return "", false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '_', '.', ':':
			continue
		default:
			return "", false
		}
	}
	return value, true
}
