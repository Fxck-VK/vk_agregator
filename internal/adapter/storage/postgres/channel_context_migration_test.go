package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChannelContextAndResultModeMigrationIsForwardSafe(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	upRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000044_channel_context_and_result_mode.up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	up := compactSQL(string(upRaw))
	for _, required := range []string{
		"alter table jobs add column if not exists channel text",
		"add column if not exists recipient_ref text",
		"add column if not exists thread_ref text",
		"add column if not exists result_mode text not null default 'legacy_unknown'",
		"add column if not exists target_channel text",
		"add column if not exists target_recipient_ref text",
		"add column if not exists target_thread_ref text",
		"alter table deliveries add column if not exists account_id uuid",
		"add column if not exists channel text",
		"add column if not exists recipient_ref text",
		"add column if not exists thread_ref text",
		"alter table deliveries alter column user_id drop not null",
		"alter column vk_peer_id drop not null",
		"alter column vk_random_id drop not null",
		"result_mode in ('external_push', 'account_history', 'legacy_unknown')",
		"target_channel = 'vk_bot'",
		"result_mode <> 'legacy_unknown' or ( target_channel is null and target_recipient_ref is null and target_thread_ref is null )",
		"channel = 'vk_bot'",
		"not valid",
		"where account_id is not null",
		"where result_mode = 'account_history'",
		"where result_mode = 'external_push'",
		"where channel is not null",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"begin;", "commit;", "drop column", "delete from", "create index concurrently"} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("up migration must not contain %q", forbidden)
		}
	}
	if strings.Contains(up, "target_channel in ('vk_bot', 'vk_miniapp', 'web')") {
		t.Fatal("up migration must not allow non-publishable channels as job delivery targets")
	}
	for _, required := range []string{
		"and source like 'vk%'",
		"and vk_peer_id is not null",
		"and vk_peer_id <> 0",
		"and target_channel is null",
		"and target_recipient_ref is null",
		"and target_thread_ref is null",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("legacy VK backfill must preserve targetless rows, missing %q", required)
		}
	}

	downRaw, err := os.ReadFile(filepath.Join(root, "migrations", "000044_channel_context_and_result_mode.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	down := compactSQL(string(downRaw))
	for _, required := range []string{
		"drop constraint if exists jobs_result_mode_known_check",
		"drop constraint if exists jobs_channel_context_shape_check",
		"drop constraint if exists jobs_delivery_target_shape_check",
		"drop constraint if exists jobs_result_mode_shape_check",
		"drop constraint if exists deliveries_target_shape_check",
		"drop index if exists jobs_account_history_owner_created_idx",
		"drop index if exists jobs_external_push_target_created_idx",
		"drop index if exists deliveries_account_id_created_idx",
		"drop index if exists deliveries_channel_created_idx",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"begin;", "commit;", "drop column", "delete from", "update "} {
		if strings.Contains(down, forbidden) {
			t.Fatalf("down migration must preserve data and omit %q", forbidden)
		}
	}
}

func compactSQL(raw string) string {
	return strings.Join(strings.Fields(strings.ToLower(raw)), " ")
}
