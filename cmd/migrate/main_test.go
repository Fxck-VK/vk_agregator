package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/platform/config"
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

func TestMigrationTimeoutRequiresPositiveDuration(t *testing.T) {
	for _, tt := range []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{name: "positive", timeout: 30 * time.Minute},
		{name: "zero", timeout: 0, wantErr: true},
		{name: "negative", timeout: -time.Second, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := migrationTimeout(config.Config{MigrationTimeout: tt.timeout})
			if tt.wantErr {
				if err == nil {
					t.Fatal("migrationTimeout() error = nil, want validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationTimeout() error = %v", err)
			}
			if got != tt.timeout {
				t.Fatalf("migrationTimeout() = %s, want %s", got, tt.timeout)
			}
		})
	}
}

func TestMigrationExecutionModeRequiresExplicitLeadingDirective(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    migrationExecutionMode
		wantErr string
	}{
		{
			name: "ordinary migration stays transactional",
			sql:  "CREATE TABLE example (id BIGINT);\n",
			want: migrationExecutionTransactional,
		},
		{
			name: "leading directive enables concurrent index migration",
			sql: "-- migrate: no-transaction\n" +
				"CREATE INDEX CONCURRENTLY example_idx ON example (id);\n",
			want: migrationExecutionNonTransactional,
		},
		{
			name:    "concurrent index without directive is rejected",
			sql:     "CREATE INDEX CONCURRENTLY example_idx ON example (id);\n",
			wantErr: "requires -- migrate: no-transaction",
		},
		{
			name: "directive after SQL is rejected rather than silently ignored",
			sql: "CREATE INDEX CONCURRENTLY example_idx ON example (id);\n" +
				"-- migrate: no-transaction\n",
			wantErr: "first non-empty line",
		},
		{
			name: "non transactional directive requires concurrent index DDL",
			sql: "-- migrate: no-transaction\n" +
				"CREATE TABLE example (id BIGINT);\n",
			wantErr: "CONCURRENTLY",
		},
		{
			name: "non transactional directive rejects mixed arbitrary DDL",
			sql: "-- migrate: no-transaction\n" +
				"CREATE TABLE example (id BIGINT);\n" +
				"CREATE INDEX CONCURRENTLY example_idx ON example (id);\n",
			wantErr: "only CREATE INDEX CONCURRENTLY or DROP INDEX CONCURRENTLY",
		},
		{
			name: "non transactional directive rejects explicit transaction control",
			sql: "-- migrate: no-transaction\n" +
				"BEGIN;\n" +
				"DROP INDEX CONCURRENTLY IF EXISTS example_idx;\n",
			wantErr: "only CREATE INDEX CONCURRENTLY or DROP INDEX CONCURRENTLY",
		},
		{
			name: "non transactional directive permits retry safe drop and create",
			sql: "-- migrate: no-transaction\n" +
				"DROP INDEX CONCURRENTLY IF EXISTS example_idx;\n" +
				"CREATE UNIQUE INDEX CONCURRENTLY example_idx ON example (id);\n",
			want: migrationExecutionNonTransactional,
		},
		{
			name:    "unknown migrate directive is rejected",
			sql:     "-- migrate: unsafe\nCREATE TABLE example (id BIGINT);\n",
			wantErr: "unsupported migration directive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := migrationExecutionModeForSQL([]byte(tt.sql))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("migrationExecutionModeForSQL() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationExecutionModeForSQL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("migrationExecutionModeForSQL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNonTransactionalMigrationSplitsConcurrentDDLIntoAutocommitStatements(t *testing.T) {
	statements, err := splitMigrationSQLStatements(`-- migrate: no-transaction
DROP INDEX CONCURRENTLY IF EXISTS example_idx;
CREATE INDEX CONCURRENTLY example_idx ON example (id);`)
	if err != nil {
		t.Fatalf("split migration: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("statement count = %d, want 2", len(statements))
	}
	if !strings.Contains(statements[0], "DROP INDEX CONCURRENTLY") || !strings.Contains(statements[1], "CREATE INDEX CONCURRENTLY") {
		t.Fatalf("statements = %#v, want separate concurrent DROP and CREATE", statements)
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

func TestConcurrentIndexMigrationsAreExplicitAndRetrySafe(t *testing.T) {
	for _, tc := range []struct {
		version string
		index   string
	}{
		{version: "000047_web_image_history_cursor", index: "jobs_web_image_history_cursor_idx"},
		{version: "000048_web_image_prepared_capacity", index: "jobs_web_image_prepared_capacity_idx"},
		{version: "000049_web_image_prepared_expiry_reconciliation", index: "jobs_web_image_prepared_expiry_reconciliation_idx"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			upSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", tc.version+".up.sql"))
			if err != nil {
				t.Fatalf("read up migration: %v", err)
			}
			mode, err := migrationExecutionModeForSQL(upSQL)
			if err != nil {
				t.Fatalf("validate up migration: %v", err)
			}
			if mode != migrationExecutionNonTransactional {
				t.Fatalf("up migration mode = %v, want non-transactional", mode)
			}
			upText := string(upSQL)
			for _, required := range []string{
				"DROP INDEX CONCURRENTLY IF EXISTS " + tc.index,
				"CREATE INDEX CONCURRENTLY " + tc.index,
			} {
				if !strings.Contains(upText, required) {
					t.Fatalf("up migration must contain retry-safe %q", required)
				}
			}

			downSQL, err := os.ReadFile(filepath.Join("..", "..", "migrations", tc.version+".down.sql"))
			if err != nil {
				t.Fatalf("read down migration: %v", err)
			}
			mode, err = migrationExecutionModeForSQL(downSQL)
			if err != nil {
				t.Fatalf("validate down migration: %v", err)
			}
			if mode != migrationExecutionNonTransactional {
				t.Fatalf("down migration mode = %v, want non-transactional", mode)
			}
			if required := "DROP INDEX CONCURRENTLY IF EXISTS " + tc.index; !strings.Contains(string(downSQL), required) {
				t.Fatalf("down migration must contain retry-safe %q", required)
			}
		})
	}
}

func TestAccountFirstRollbackMigrationsProtectPersistedData(t *testing.T) {
	for _, tc := range []struct {
		name      string
		required  []string
		forbidden []string
	}{
		{
			name: "000044_channel_context_and_result_mode.down.sql",
			required: []string{
				"cannot roll back 000044",
				"ALTER COLUMN user_id SET NOT NULL",
			},
			forbidden: []string{
				"drop column",
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
			rawText := strings.ToLower(string(raw))
			for _, fragment := range tc.required {
				if !strings.Contains(rawText, strings.ToLower(fragment)) {
					t.Errorf("rollback migration must contain %q", fragment)
				}
			}
			for _, fragment := range tc.forbidden {
				if strings.Contains(rawText, strings.ToLower(fragment)) {
					t.Errorf("rollback migration must preserve additive schema and omit %q", fragment)
				}
			}
		})
	}
}
