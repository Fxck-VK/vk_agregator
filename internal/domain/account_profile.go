package domain

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxAccountDisplayNameRunes = 160

// AccountProfileRepository stores presentation data on the canonical account.
// The initializer accepts only an owned, verified VK/Yandex binding and never replaces
// an existing name (including a name set by the account owner).
type AccountProfileRepository interface {
	GetAccountDisplayName(context.Context, uuid.UUID) (string, error)
	SetAccountDisplayNameIfEmpty(context.Context, uuid.UUID, uuid.UUID, string) error
}

// NormalizeAccountDisplayName bounds provider metadata and removes formatting
// controls. Invalid/oversized names are ignored without preventing sign-in.
func NormalizeAccountDisplayName(name string) string {
	if !utf8.ValidString(name) {
		return ""
	}
	name = strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	}), " ")
	if utf8.RuneCountInString(name) > MaxAccountDisplayNameRunes {
		return ""
	}
	return name
}
