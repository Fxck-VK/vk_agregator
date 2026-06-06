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
