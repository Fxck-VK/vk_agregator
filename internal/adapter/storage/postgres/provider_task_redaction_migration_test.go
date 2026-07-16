package postgres_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderTaskPayloadRedactionMigrationIsAdditiveAndIdempotent(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	upRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000040_provider_task_payload_redaction.up.sql"))
	if err != nil {
		t.Fatalf("read provider task redaction migration: %v", err)
	}
	up := strings.ToLower(string(upRaw))
	for _, required := range []string{
		"with sanitized as",
		"update provider_tasks",
		"jsonb_build_object",
		"'status'",
		"'error_class'",
		"redacted_at",
		"request = '{}'::jsonb",
		"is distinct from sanitized.safe_result",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("redaction migration missing %q", required)
		}
	}
	if strings.Contains(up, "request ?|") || strings.Contains(up, "result ?|") {
		t.Fatalf("redaction migration must not depend on a finite payload-key denylist")
	}
	for _, forbidden := range []string{
		"drop table",
		"drop column",
		"truncate",
		"delete from provider_tasks",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("redaction migration must not use destructive statement %q", forbidden)
		}
	}

	downRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000040_provider_task_payload_redaction.down.sql"))
	if err != nil {
		t.Fatalf("read provider task redaction down migration: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(downRaw)), "no-op") {
		t.Fatalf("down migration must declare irreversible no-op redaction")
	}
}
