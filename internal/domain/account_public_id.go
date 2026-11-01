package domain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// AccountPublicIDRepository resolves permanent support identifiers. Public IDs
// are labels, never credentials or substitutes for authenticated ownership.
type AccountPublicIDRepository interface {
	GetAccountPublicNumber(context.Context, uuid.UUID) (int64, error)
	GetAccountByPublicNumber(context.Context, int64) (*Account, error)
}

func FormatAccountPublicID(number int64) string {
	if number <= 0 {
		return ""
	}
	return fmt.Sprintf("EP-%06d", number)
}

func ParseAccountPublicID(value string) (int64, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) < 9 || len(value) > 22 || !strings.HasPrefix(value, "EP-") {
		return 0, ErrInvalidIdentity
	}
	for _, digit := range value[3:] {
		if digit < '0' || digit > '9' {
			return 0, ErrInvalidIdentity
		}
	}
	number, err := strconv.ParseInt(value[3:], 10, 64)
	if err != nil || number <= 0 || FormatAccountPublicID(number) != value {
		return 0, ErrInvalidIdentity
	}
	return number, nil
}
