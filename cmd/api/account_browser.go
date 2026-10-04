package main

import "strings"

// The first configured audience is the browser client; the others remain valid
// for existing mobile assertion adapters. No public request selects an audience.
func firstOAuthClientID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.TrimSpace(ids[0])
}
