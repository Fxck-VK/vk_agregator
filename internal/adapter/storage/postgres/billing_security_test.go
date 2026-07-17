package postgres

import (
	"strings"
	"testing"
)

func TestCreditAccountOwnerFilterDoesNotCrossCanonicalOwners(t *testing.T) {
	compact := strings.Join(strings.Fields(creditAccountOwnerFilter), " ")
	if compact != "(owner_account_id = $1 OR (owner_account_id IS NULL AND user_id = $1))" {
		t.Fatalf("unsafe credit account owner filter: %s", compact)
	}
}
