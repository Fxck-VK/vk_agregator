package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChecksumNormalizesLineEndings(t *testing.T) {
	lf := []byte("CREATE TABLE example (id BIGINT);\nSELECT 1;\n")
	crlf := []byte("CREATE TABLE example (id BIGINT);\r\nSELECT 1;\r\n")

	if got, want := checksum(crlf), checksum(lf); got != want {
		t.Fatalf("checksum differs by line endings: got %s, want %s", got, want)
	}
}

func TestChecksumMatchesLegacyCRLF(t *testing.T) {
	lf := []byte("CREATE TABLE example (id BIGINT);\nSELECT 1;\n")
	crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	legacy := fmt.Sprintf("%x", sha256.Sum256(crlf))

	if !checksumMatches(legacy, lf) {
		t.Fatalf("legacy CRLF checksum %s was rejected", legacy)
	}
}

func TestChecksumMatchesRejectsSQLDrift(t *testing.T) {
	recorded := checksum([]byte("SELECT 1;\n"))

	if checksumMatches(recorded, []byte("SELECT 2;\n")) {
		t.Fatal("checksum accepted changed SQL")
	}
}

func TestMigrationFilesDelegateTransactionsToRunner(t *testing.T) {
	for _, name := range []string{
		"000042_account_session_access_tokens.up.sql",
		"000042_account_session_access_tokens.down.sql",
		"000043_account_native_legacy_nullable.up.sql",
		"000043_account_native_legacy_nullable.down.sql",
		"000044_channel_context_and_result_mode.up.sql",
		"000044_channel_context_and_result_mode.down.sql",
		"000045_outbox_claim_lease.up.sql",
		"000045_outbox_claim_lease.down.sql",
		"000046_web_account_conversations.up.sql",
		"000046_web_account_conversations.down.sql",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
			if err != nil {
				t.Fatalf("read migration: %v", err)
			}
			for lineNumber, line := range strings.Split(strings.ToLower(string(raw)), "\n") {
				statement := strings.TrimSpace(line)
				if statement == "begin;" || statement == "commit;" {
					t.Fatalf("line %d contains %q; cmd/migrate must own the transaction", lineNumber+1, statement)
				}
			}
		})
	}
}

func TestAccountFirstRollbackMigrationsProtectPersistedData(t *testing.T) {
	for _, tc := range []struct {
		name     string
		required []string
	}{
		{
			name: "000044_channel_context_and_result_mode.down.sql",
			required: []string{
				"cannot roll back 000044",
				"DROP COLUMN IF EXISTS result_mode",
				"ALTER COLUMN user_id SET NOT NULL",
			},
		},
		{
			name: "000045_outbox_claim_lease.down.sql",
			required: []string{
				"cannot roll back 000045",
				"claim_token IS NOT NULL",
				"last_error_code <> ''",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", tc.name))
			if err != nil {
				t.Fatalf("read migration: %v", err)
			}
			for _, fragment := range tc.required {
				if !strings.Contains(string(raw), fragment) {
					t.Errorf("rollback migration must contain %q", fragment)
				}
			}
		})
	}
}
