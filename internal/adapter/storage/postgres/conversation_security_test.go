package postgres

import (
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
