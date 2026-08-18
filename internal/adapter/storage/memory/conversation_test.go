package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestConversationRepoVKBotLookupRemainsBackwardCompatible(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	userID := uuid.New()

	conv := &domain.Conversation{
		UserID:   userID,
		VKPeerID: 42,
	}
	if err := repo.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	got, err := repo.GetActiveByUserPeer(ctx, userID, 42)
	if err != nil {
		t.Fatalf("get by user peer: %v", err)
	}
	if got.ID != conv.ID || got.Source != domain.ConversationSourceVKBot {
		t.Fatalf("conversation = (%s, %s), want (%s, %s)", got.ID, got.Source, conv.ID, domain.ConversationSourceVKBot)
	}

	byRef, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
		UserID:   userID,
		Source:   domain.ConversationSourceVKBot,
		VKPeerID: 42,
	})
	if err != nil {
		t.Fatalf("get by reference: %v", err)
	}
	if byRef.ID != conv.ID {
		t.Fatalf("reference id = %s, want %s", byRef.ID, conv.ID)
	}
}

func TestConversationRepoMiniAppThreadsAreIsolated(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	userID := uuid.New()

	threadA := &domain.Conversation{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-a",
		Title:            "A",
	}
	threadB := &domain.Conversation{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-b",
		Title:            "B",
	}
	if err := repo.CreateConversation(ctx, threadA); err != nil {
		t.Fatalf("create thread a: %v", err)
	}
	if err := repo.CreateConversation(ctx, threadB); err != nil {
		t.Fatalf("create thread b: %v", err)
	}

	gotA, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-a",
	})
	if err != nil {
		t.Fatalf("get thread a: %v", err)
	}
	gotB, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-b",
	})
	if err != nil {
		t.Fatalf("get thread b: %v", err)
	}
	if gotA.ID == gotB.ID {
		t.Fatalf("miniapp threads share one conversation id: %s", gotA.ID)
	}

	duplicate := &domain.Conversation{
		UserID:           userID,
		Source:           domain.ConversationSourceMiniApp,
		ExternalThreadID: "thread-a",
	}
	if err := repo.CreateConversation(ctx, duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate error = %v, want %v", err, domain.ErrConflict)
	}

	listed, err := repo.ListByUserSource(ctx, userID, domain.ConversationSourceMiniApp, 10, 0)
	if err != nil {
		t.Fatalf("list miniapp threads: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed threads = %d, want 2", len(listed))
	}
}

func TestConversationRepoWebThreadsUseExactAccountOwner(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	accountA := uuid.New()
	accountB := uuid.New()

	webA := &domain.Conversation{
		UserID:           uuid.Nil,
		AccountID:        accountA,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "shared-thread",
	}
	webB := &domain.Conversation{
		UserID:           uuid.Nil,
		AccountID:        accountB,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "shared-thread",
	}
	legacyUserMatch := &domain.Conversation{
		UserID:           accountA,
		AccountID:        accountB,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "legacy-user-match",
	}
	for _, conversation := range []*domain.Conversation{webA, webB, legacyUserMatch} {
		if err := repo.CreateConversation(ctx, conversation); err != nil {
			t.Fatalf("create web conversation: %v", err)
		}
	}
	if webA.UserID != uuid.Nil {
		t.Fatalf("account-only web conversation user id = %s, want nil", webA.UserID)
	}

	got, err := repo.GetByIDForAccount(ctx, accountA, webA.ID)
	if err != nil {
		t.Fatalf("get account a web conversation: %v", err)
	}
	if got.ID != webA.ID || got.UserID != uuid.Nil || got.AccountID != accountA {
		t.Fatalf("account a conversation = %#v, want account-only web conversation", got)
	}
	if _, err := repo.GetByIDForAccount(ctx, accountB, webA.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign account get error = %v, want %v", err, domain.ErrNotFound)
	}
	if _, err := repo.GetByIDForAccount(ctx, accountA, legacyUserMatch.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy user fallback get error = %v, want %v", err, domain.ErrNotFound)
	}

	listed, err := repo.ListByAccountSource(ctx, accountA, domain.ConversationSourceWeb, 10, 0)
	if err != nil {
		t.Fatalf("list account a web conversations: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != webA.ID {
		t.Fatalf("account a web conversations = %#v, want only %s", listed, webA.ID)
	}
}

func TestSetGeneratedTitleDoesNotOverwriteManualRename(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	accountID := uuid.New()
	conversation := &domain.Conversation{
		AccountID:        accountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "title-state",
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conversation.TitleOrigin != domain.ConversationTitleOriginAutoPending {
		t.Fatalf("initial title origin = %q, want %q", conversation.TitleOrigin, domain.ConversationTitleOriginAutoPending)
	}

	applied, err := repo.SetConversationFallbackTitleIfPending(ctx, conversation.ID, "First user prompt")
	if err != nil || !applied {
		t.Fatalf("set fallback = %t, %v; want true, nil", applied, err)
	}
	renamed, err := repo.RenameActiveConversationForAccount(ctx, accountID, conversation.ID, domain.ConversationSourceWeb, "My manual name")
	if err != nil {
		t.Fatalf("rename conversation: %v", err)
	}
	if renamed.TitleOrigin != domain.ConversationTitleOriginManual {
		t.Fatalf("renamed origin = %q, want %q", renamed.TitleOrigin, domain.ConversationTitleOriginManual)
	}

	applied, err = repo.SetGeneratedTitleForActiveWebConversation(ctx, accountID, conversation.ID, "Semantic model title")
	if err != nil {
		t.Fatalf("set generated title: %v", err)
	}
	if applied {
		t.Fatal("generated title overwrote a manual rename")
	}
	stored, err := repo.GetByIDForAccount(ctx, accountID, conversation.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if stored.Title != "My manual name" || stored.TitleOrigin != domain.ConversationTitleOriginManual {
		t.Fatalf("stored conversation = %#v, want manual title and origin", stored)
	}
}

func TestSetConversationFallbackTitleSkipsArchivedWebConversation(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	conversation := &domain.Conversation{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		AccountID:   uuid.New(),
		Source:      domain.ConversationSourceWeb,
		Status:      domain.ConversationArchived,
		TitleOrigin: domain.ConversationTitleOriginAutoPending,
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create archived conversation: %v", err)
	}

	applied, err := repo.SetConversationFallbackTitleIfPending(ctx, conversation.ID, "Late background prompt")
	if err != nil || applied {
		t.Fatalf("set fallback title = %t, %v; want false, nil", applied, err)
	}
	stored, err := repo.GetByIDForAccount(ctx, conversation.AccountID, conversation.ID)
	if err != nil {
		t.Fatalf("get archived conversation: %v", err)
	}
	if stored.Title != "" || stored.TitleOrigin != domain.ConversationTitleOriginAutoPending {
		t.Fatalf("archived title state = %#v, want unchanged pending state", stored)
	}
}

func TestConversationRepoFindsUserMessagesForTitleWork(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	conversation := &domain.Conversation{
		AccountID:        uuid.New(),
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "title-messages",
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	firstJobID := uuid.New()
	first, err := repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          firstJobID,
		Role:           domain.ConversationRoleUser,
		Text:           "First prompt",
	})
	if err != nil {
		t.Fatalf("insert first user message: %v", err)
	}
	if _, err := repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          uuid.New(),
		Role:           domain.ConversationRoleUser,
		Text:           "Second prompt",
	}); err != nil {
		t.Fatalf("insert second user message: %v", err)
	}

	byJob, err := repo.GetUserMessageByJobID(ctx, firstJobID)
	if err != nil || byJob.ID != first.ID || byJob.Text != "First prompt" {
		t.Fatalf("user message by job = %#v, %v", byJob, err)
	}
	firstUser, err := repo.GetFirstUserMessage(ctx, conversation.ID)
	if err != nil || firstUser.ID != first.ID || firstUser.JobID != firstJobID {
		t.Fatalf("first user message = %#v, %v", firstUser, err)
	}
}

func TestConversationRepoRatesOnlyOwnedActiveWebAssistantMessages(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	accountID := uuid.New()
	conversation := &domain.Conversation{
		AccountID:        accountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "rated-thread",
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	assistant, err := repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          uuid.New(),
		Role:           domain.ConversationRoleAssistant,
		Text:           "Answer",
	})
	if err != nil {
		t.Fatalf("insert assistant: %v", err)
	}
	user, err := repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          uuid.New(),
		Role:           domain.ConversationRoleUser,
		Text:           "Question",
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	rated, err := repo.SetMessageRatingForAccount(
		ctx,
		accountID,
		conversation.ID,
		assistant.ID,
		domain.ConversationSourceWeb,
		domain.ConversationMessageRatingLike,
	)
	if err != nil || rated.Rating != domain.ConversationMessageRatingLike {
		t.Fatalf("rate assistant = %#v, %v", rated, err)
	}
	messages, err := repo.ListMessagesAfter(ctx, conversation.ID, 0, 10)
	if err != nil || len(messages) != 2 || messages[0].Rating != domain.ConversationMessageRatingLike {
		t.Fatalf("persisted rating messages = %#v, %v", messages, err)
	}
	cleared, err := repo.SetMessageRatingForAccount(
		ctx,
		accountID,
		conversation.ID,
		assistant.ID,
		domain.ConversationSourceWeb,
		domain.ConversationMessageRatingNone,
	)
	if err != nil || cleared.Rating != domain.ConversationMessageRatingNone {
		t.Fatalf("clear assistant rating = %#v, %v", cleared, err)
	}

	for name, testCase := range map[string]struct {
		accountID uuid.UUID
		messageID uuid.UUID
		source    domain.ConversationSource
	}{
		"foreign account": {uuid.New(), assistant.ID, domain.ConversationSourceWeb},
		"user message":    {accountID, user.ID, domain.ConversationSourceWeb},
		"wrong source":    {accountID, assistant.ID, domain.ConversationSourceMiniApp},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := repo.SetMessageRatingForAccount(ctx, testCase.accountID, conversation.ID, testCase.messageID, testCase.source, domain.ConversationMessageRatingDislike); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("rating error = %v, want %v", err, domain.ErrNotFound)
			}
		})
	}
}

func TestConversationRepoRejectsUnownedWebConversation(t *testing.T) {
	repo := NewConversationRepo()
	err := repo.CreateConversation(context.Background(), &domain.Conversation{
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "unowned-web-thread",
	})
	if !errors.Is(err, domain.ErrConversationAccountOwnershipRequired) {
		t.Fatalf("create unowned web conversation error = %v, want %v", err, domain.ErrConversationAccountOwnershipRequired)
	}
}

func TestConversationRepoArchiveDualOwnerRemovesAccountAndLegacyActiveReferences(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	userID := uuid.New()
	accountID := uuid.New()
	conversation := &domain.Conversation{
		UserID:           userID,
		AccountID:        accountID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: "dual-owner-thread",
	}
	if err := repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create dual-owner conversation: %v", err)
	}

	if err := repo.ArchiveConversationForAccount(ctx, accountID, conversation.ID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("archive conversation: %v", err)
	}
	if err := repo.ArchiveConversationForAccount(ctx, accountID, conversation.ID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("repeat archive conversation: %v", err)
	}

	lookups := []domain.ConversationRef{
		{
			UserID:           userID,
			AccountID:        accountID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: conversation.ExternalThreadID,
		},
		{
			UserID:           userID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: conversation.ExternalThreadID,
		},
	}
	for _, ref := range lookups {
		if got, err := repo.GetActiveByReference(ctx, ref); !errors.Is(err, domain.ErrNotFound) || got != nil {
			t.Fatalf("archived active lookup for account=%s user=%s = %#v, %v; want nil, %v", ref.AccountID, ref.UserID, got, err, domain.ErrNotFound)
		}
	}

	stored, err := repo.GetByIDForAccount(ctx, accountID, conversation.ID)
	if err != nil {
		t.Fatalf("read archived history: %v", err)
	}
	if stored.Status != domain.ConversationArchived {
		t.Fatalf("stored status = %q, want %q", stored.Status, domain.ConversationArchived)
	}
	if _, err := repo.GetByIDForAccount(ctx, uuid.New(), conversation.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign account history lookup = %v, want %v", err, domain.ErrNotFound)
	}
}

func TestConversationRepoArchiveKeepsLegacyReferenceReownedByAnotherAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewConversationRepo()
	userID := uuid.New()
	accountA := uuid.New()
	accountB := uuid.New()
	const threadID = "shared-legacy-thread"
	conversationA := &domain.Conversation{
		UserID:           userID,
		AccountID:        accountA,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: threadID,
	}
	conversationB := &domain.Conversation{
		UserID:           userID,
		AccountID:        accountB,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: threadID,
	}
	for _, conversation := range []*domain.Conversation{conversationA, conversationB} {
		if err := repo.CreateConversation(ctx, conversation); err != nil {
			t.Fatalf("create conversation: %v", err)
		}
	}

	legacyRef := domain.ConversationRef{
		UserID:           userID,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: threadID,
	}
	if got, err := repo.GetActiveByReference(ctx, legacyRef); err != nil || got.ID != conversationB.ID {
		t.Fatalf("legacy active lookup after B creation = %#v, %v; want %s, nil", got, err, conversationB.ID)
	}

	if err := repo.ArchiveConversationForAccount(ctx, accountA, conversationA.ID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("archive A: %v", err)
	}
	if got, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
		AccountID:        accountA,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: threadID,
	}); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("A account active lookup after archive = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
	}
	if got, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
		UserID:           userID,
		AccountID:        accountA,
		Source:           domain.ConversationSourceWeb,
		ExternalThreadID: threadID,
	}); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("A account-aware active lookup after archive = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
	}
	if got, err := repo.GetActiveByReference(ctx, legacyRef); err != nil || got.ID != conversationB.ID || got.Status != domain.ConversationActive {
		t.Fatalf("legacy active lookup after A archive = %#v, %v; want active B %s", got, err, conversationB.ID)
	}
	if err := repo.ArchiveConversationForAccount(ctx, accountB, conversationB.ID, domain.ConversationSourceWeb); err != nil {
		t.Fatalf("archive B: %v", err)
	}
	if got, err := repo.GetActiveByReference(ctx, legacyRef); !errors.Is(err, domain.ErrNotFound) || got != nil {
		t.Fatalf("legacy active lookup after B archive = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
	}
}

func TestConversationRepoAccountAwareReferenceUsesOnlyExactOrHistoricalFallback(t *testing.T) {
	ctx := context.Background()

	t.Run("falls back to matching historical user", func(t *testing.T) {
		repo := NewConversationRepo()
		userID := uuid.New()
		accountID := uuid.New()
		conversation := domain.Conversation{
			ID:               uuid.New(),
			UserID:           userID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: "historical-fallback",
			Status:           domain.ConversationActive,
		}
		seedActiveConversationReference(t, repo, conversation, domain.ConversationRef{
			UserID:           userID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})

		got, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
			UserID:           userID,
			AccountID:        accountID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})
		if err != nil || got.ID != conversation.ID {
			t.Fatalf("account-aware historical fallback = %#v, %v; want %s, nil", got, err, conversation.ID)
		}
	})

	t.Run("rejects canonical same-account candidate reached only through legacy key", func(t *testing.T) {
		repo := NewConversationRepo()
		userID := uuid.New()
		accountID := uuid.New()
		conversation := domain.Conversation{
			ID:               uuid.New(),
			UserID:           userID,
			AccountID:        accountID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: "canonical-legacy-only",
			Status:           domain.ConversationActive,
		}
		seedActiveConversationReference(t, repo, conversation, domain.ConversationRef{
			UserID:           userID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})

		got, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
			UserID:           userID,
			AccountID:        accountID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})
		if !errors.Is(err, domain.ErrNotFound) || got != nil {
			t.Fatalf("canonical legacy fallback = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
		}
	})

	t.Run("rejects historical candidate reached through a colliding direct account key", func(t *testing.T) {
		repo := NewConversationRepo()
		requestedUserID := uuid.New()
		requestedAccountID := uuid.New()
		conversation := domain.Conversation{
			ID:               uuid.New(),
			UserID:           requestedAccountID,
			Source:           domain.ConversationSourceWeb,
			ExternalThreadID: "direct-account-collision",
			Status:           domain.ConversationActive,
		}
		seedActiveConversationReference(t, repo, conversation, domain.ConversationRef{
			AccountID:        requestedAccountID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})

		got, err := repo.GetActiveByReference(ctx, domain.ConversationRef{
			UserID:           requestedUserID,
			AccountID:        requestedAccountID,
			Source:           conversation.Source,
			ExternalThreadID: conversation.ExternalThreadID,
		})
		if !errors.Is(err, domain.ErrNotFound) || got != nil {
			t.Fatalf("direct historical account collision = %#v, %v; want nil, %v", got, err, domain.ErrNotFound)
		}
	})
}

func seedActiveConversationReference(t *testing.T, repo *ConversationRepo, conversation domain.Conversation, ref domain.ConversationRef) {
	t.Helper()
	if conversation.ID == uuid.Nil || conversation.Status != domain.ConversationActive {
		t.Fatalf("invalid active conversation fixture: %#v", conversation)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.byID[conversation.ID] = conversation
	repo.activeByRef[activeConversationRefKey(ref)] = conversation.ID
}
