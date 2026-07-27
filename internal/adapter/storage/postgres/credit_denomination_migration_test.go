package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStarDenominationMigrationIsAppendOnlyAndIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "..", "migrations", "000041_star_denomination.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(data))

	required := []string{
		"credit_denomination_version",
		"insert into ledger_entries",
		"'denomination:v2:'",
		"on conflict (idempotency_key) do nothing",
		"'crystals_10_dev'",
		"then 20",
		"'neirohub dev 20 stars'",
		"'crystals_99'",
		"then 198",
		"'neirohub 198 stars'",
		"'crystals_150'",
		"then 300",
		"'neirohub 300 stars'",
		"'crystals_250'",
		"then 500",
		"'neirohub 500 stars'",
		"'crystals_400'",
		"then 800",
		"'neirohub 800 stars'",
		"'crystals_700'",
		"then 1400",
		"'neirohub 1400 stars'",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}

	forbidden := []string{
		"delete from ledger_entries",
		"truncate ledger_entries",
		"update ledger_entries set amount",
		"drop table",
	}
	for _, fragment := range forbidden {
		if strings.Contains(sql, fragment) {
			t.Fatalf("migration contains forbidden operation %q", fragment)
		}
	}
}
