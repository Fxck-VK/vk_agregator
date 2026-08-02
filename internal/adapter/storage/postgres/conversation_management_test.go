package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestWebConversationManagementQueriesUseExactActiveAccountScope(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	conversationID := uuid.New()

	t.Run("active list", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewConversationRepository(recorder).ListActiveByAccountSource(ctx, accountID, domain.ConversationSourceWeb, 10, 0)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("list error = %v, want %v", err, domain.ErrNotFound)
		}
		assertWebConversationManagementQuery(t, recorder.query, "WHERE account_id = $1 AND source = $2 AND status = 'active'")
		if len(recorder.args) != 4 || recorder.args[0] != accountID || recorder.args[1] != domain.ConversationSourceWeb || recorder.args[2] != 10 || recorder.args[3] != 0 {
			t.Fatalf("list args = %#v", recorder.args)
		}
	})

	t.Run("rename active", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		_, err := NewConversationRepository(recorder).RenameActiveConversationForAccount(ctx, accountID, conversationID, domain.ConversationSourceWeb, "New title")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("rename error = %v, want %v", err, domain.ErrNotFound)
		}
		assertWebConversationManagementQuery(t, recorder.query, "WHERE id = $1 AND account_id = $2 AND source = $3 AND status = 'active'")
		if len(recorder.args) != 4 || recorder.args[0] != conversationID || recorder.args[1] != accountID || recorder.args[2] != domain.ConversationSourceWeb || recorder.args[3] != "New title" {
			t.Fatalf("rename args = %#v", recorder.args)
		}
	})

	t.Run("archive is idempotent for active and archived rows only", func(t *testing.T) {
		recorder := &scopedQueryRecorder{}
		err := NewConversationRepository(recorder).ArchiveConversationForAccount(ctx, accountID, conversationID, domain.ConversationSourceWeb)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("archive error = %v, want %v", err, domain.ErrNotFound)
		}
		assertWebConversationManagementQuery(t, recorder.query, "WHERE id = $1 AND account_id = $2 AND source = $3 AND status IN ('active', 'archived')")
		if len(recorder.args) != 3 || recorder.args[0] != conversationID || recorder.args[1] != accountID || recorder.args[2] != domain.ConversationSourceWeb {
			t.Fatalf("archive args = %#v", recorder.args)
		}
	})
}

func assertWebConversationManagementQuery(t *testing.T, query, required string) {
	t.Helper()
	compact := strings.Join(strings.Fields(query), " ")
	if !strings.Contains(compact, required) || strings.Contains(compact, "OR user_id") || strings.Contains(compact, "OR (account_id") {
		t.Fatalf("query must use exact account/source ownership: %s", compact)
	}
}
