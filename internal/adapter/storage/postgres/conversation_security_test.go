package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestActiveConversationLookupScopesCanonicalAccountBeforeLegacyUser(t *testing.T) {
	accountID := uuid.New()
	userID := uuid.New()

	query, args := activeConversationLookup(domain.ConversationRef{
		AccountID: accountID,
		UserID:    userID,
		Source:    domain.ConversationSourceVKBot,
		VKPeerID:  42,
	})

	compact := strings.Join(strings.Fields(query), " ")
	if !strings.Contains(compact, "account_id = $1 OR (account_id IS NULL AND user_id = $2)") {
		t.Fatalf("canonical account scope missing from query: %s", compact)
	}
	if strings.Contains(compact, "user_id = $1 OR account_id = $1") {
		t.Fatalf("query ambiguously treats one id as account and user: %s", compact)
	}
	if len(args) != 3 || args[0] != accountID || args[1] != userID || args[2] != int64(42) {
		t.Fatalf("unexpected query args: %#v", args)
	}
}

func TestActiveConversationLookupUsesLegacyUserOnlyWithoutAccount(t *testing.T) {
	userID := uuid.New()

	query, args := activeConversationLookup(domain.ConversationRef{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-1",
	})

	compact := strings.Join(strings.Fields(query), " ")
	if !strings.Contains(compact, "WHERE user_id = $1") || strings.Contains(compact, "account_id = $1") {
		t.Fatalf("legacy lookup must stay scoped to user_id: %s", compact)
	}
	if len(args) != 3 || args[0] != userID || args[1] != domain.ConversationSourceMiniApp || args[2] != "thread-1" {
		t.Fatalf("unexpected query args: %#v", args)
	}
}

func TestWebConversationRepositoryUsesExactAccountOwnerAndNullLegacyUser(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	conversationID := uuid.New()

	t.Run("get by id", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewConversationRepository(recorder).GetByIDForAccount(ctx, accountID, conversationID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("get error = %v, want %v", err, domain.ErrNotFound)
		}
		assertExactConversationAccountQuery(t, recorder.query, "WHERE account_id = $1 AND id = $2")
	})

	t.Run("list", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewConversationRepository(recorder).ListByAccountSource(ctx, accountID, domain.ConversationSourceWeb, 10, 0)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("list error = %v, want %v", err, domain.ErrNotFound)
		}
		assertExactConversationAccountQuery(t, recorder.query, "WHERE account_id = $1 AND source = $2")
	})

	t.Run("create account-only row", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewConversationRepository(recorder).CreateConversation(ctx, &domain.Conversation{
			AccountID:        accountID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: "web-thread",
		})
		assertNullableRecorderError(t, err)
		assertNilArgument(t, recorder.args, 1, "conversation user_id")
		if len(recorder.args) <= 2 || !reflect.DeepEqual(recorder.args[2], &accountID) {
			t.Fatalf("conversation account id argument = %#v, want %s", recorder.args, accountID)
		}
	})

	t.Run("reject unowned web row before query", func(t *testing.T) {
		recorder := &nullableRecorder{}
		err := NewConversationRepository(recorder).CreateConversation(ctx, &domain.Conversation{
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: "unowned-web-thread",
		})
		if !errors.Is(err, domain.ErrConversationAccountOwnershipRequired) {
			t.Fatalf("create unowned web conversation error = %v, want %v", err, domain.ErrConversationAccountOwnershipRequired)
		}
		if recorder.query != "" || len(recorder.args) != 0 {
			t.Fatalf("unowned web conversation reached storage: query=%q args=%#v", recorder.query, recorder.args)
		}
	})
}

func TestAccountOnlyWebConversationScanPreservesNullLegacyUser(t *testing.T) {
	accountID := uuid.New()
	var conversation domain.Conversation
	if err := scanConversation(nullableLegacyScanRow{
		uuidPointers:  map[int]uuid.UUID{2: accountID},
		nullableUUIDs: map[int]bool{1: true},
	}, &conversation); err != nil {
		t.Fatalf("scan conversation: %v", err)
	}
	if conversation.UserID != uuid.Nil || conversation.AccountID != accountID {
		t.Fatalf("conversation lost account or retained absent legacy user: %#v", conversation)
	}
}

func TestWebAccountConversationMigration(t *testing.T) {
	root, err := nullableMigrationRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	readMigration := func(t *testing.T, name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		return strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
	}

	up := readMigration(t, "000046_web_account_conversations.up.sql")
	for _, required := range []string{
		"alter table conversations alter column user_id drop not null;",
		"drop constraint if exists conversations_source_check",
		"add constraint conversations_source_check check (source in ('vk_bot', 'miniapp', 'web'));",
		"add constraint conversations_web_account_owner_check check (source <> 'web' or account_id is not null);",
		"create unique index if not exists conversations_active_account_web_thread_key",
		"on conversations (account_id, source, external_thread_id)",
		"where status = 'active' and source = 'web' and account_id is not null;",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"delete from", "update conversations", "create index concurrently"} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("up migration must not contain %q", forbidden)
		}
	}

	down := readMigration(t, "000046_web_account_conversations.down.sql")
	for _, required := range []string{
		"drop constraint if exists conversations_web_account_owner_check;",
		"drop index if exists conversations_active_account_web_thread_key;",
		"alter table conversations alter column user_id set not null;",
		"add constraint conversations_source_check check (source in ('vk_bot', 'miniapp'));",
		"raise exception",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}

func assertExactConversationAccountQuery(t *testing.T, query, ownerClause string) {
	t.Helper()
	compact := strings.Join(strings.Fields(query), " ")
	if !strings.Contains(compact, ownerClause) || strings.Contains(compact, "OR user_id") || strings.Contains(compact, "OR (account_id") {
		t.Fatalf("query must use only exact canonical account owner: %s", compact)
	}
}
