package memory

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ConversationRepo is an in-memory domain.ConversationRepository.
type ConversationRepo struct {
	mu             sync.Mutex
	nextSeq        int64
	byID           map[uuid.UUID]domain.Conversation
	activeByPeer   map[string]uuid.UUID
	messagesByID   map[uuid.UUID]domain.ConversationMessage
	messageByRole  map[string]uuid.UUID
	byConversation map[uuid.UUID][]uuid.UUID
	summaries      map[uuid.UUID]domain.ConversationSummary
}

// NewConversationRepo builds an empty ConversationRepo.
func NewConversationRepo() *ConversationRepo {
	return &ConversationRepo{
		byID:           map[uuid.UUID]domain.Conversation{},
		activeByPeer:   map[string]uuid.UUID{},
		messagesByID:   map[uuid.UUID]domain.ConversationMessage{},
		messageByRole:  map[string]uuid.UUID{},
		byConversation: map[uuid.UUID][]uuid.UUID{},
		summaries:      map[uuid.UUID]domain.ConversationSummary{},
	}
}

var _ domain.ConversationRepository = (*ConversationRepo)(nil)

func activeConversationKey(userID uuid.UUID, peerID int64) string {
	return userID.String() + "|" + strconv.FormatInt(peerID, 10)
}

func messageRoleKey(jobID uuid.UUID, role domain.ConversationMessageRole) string {
	return jobID.String() + "|" + string(role)
}

func (r *ConversationRepo) GetActiveByUserPeer(_ context.Context, userID uuid.UUID, vkPeerID int64) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.activeByPeer[activeConversationKey(userID, vkPeerID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := r.byID[id]
	return &c, nil
}

func (r *ConversationRepo) CreateConversation(_ context.Context, c *domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.Status == "" {
		c.Status = domain.ConversationActive
	}
	key := activeConversationKey(c.UserID, c.VKPeerID)
	if c.Status == domain.ConversationActive {
		if _, ok := r.activeByPeer[key]; ok {
			return domain.ErrConflict
		}
		r.activeByPeer[key] = c.ID
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	r.byID[c.ID] = *c
	return nil
}

func (r *ConversationRepo) UpsertMessage(_ context.Context, m *domain.ConversationMessage) (*domain.ConversationMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.messageByRole[messageRoleKey(m.JobID, m.Role)]; ok {
		existing := r.messagesByID[id]
		return &existing, nil
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	r.nextSeq++
	m.Seq = r.nextSeq
	m.CreatedAt = time.Now()
	r.messagesByID[m.ID] = *m
	r.messageByRole[messageRoleKey(m.JobID, m.Role)] = m.ID
	r.byConversation[m.ConversationID] = append(r.byConversation[m.ConversationID], m.ID)
	if c, ok := r.byID[m.ConversationID]; ok {
		c.UpdatedAt = m.CreatedAt
		r.byID[m.ConversationID] = c
	}
	out := *m
	return &out, nil
}

func (r *ConversationRepo) ListRecentMessagesBefore(_ context.Context, conversationID uuid.UUID, beforeSeq, minSeq int64, limit int) ([]*domain.ConversationMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matched []domain.ConversationMessage
	for _, id := range r.byConversation[conversationID] {
		m := r.messagesByID[id]
		if m.Seq < beforeSeq && m.Seq > minSeq {
			matched = append(matched, m)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Seq > matched[j].Seq })
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Seq < matched[j].Seq })
	return copyConversationMessages(matched), nil
}

func (r *ConversationRepo) ListMessagesAfter(_ context.Context, conversationID uuid.UUID, afterSeq int64, limit int) ([]*domain.ConversationMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matched []domain.ConversationMessage
	for _, id := range r.byConversation[conversationID] {
		m := r.messagesByID[id]
		if m.Seq > afterSeq {
			matched = append(matched, m)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Seq < matched[j].Seq })
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return copyConversationMessages(matched), nil
}

func (r *ConversationRepo) LatestSummary(_ context.Context, conversationID uuid.UUID) (*domain.ConversationSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.summaries[conversationID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &s, nil
}

func (r *ConversationRepo) UpsertSummary(_ context.Context, s *domain.ConversationSummary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now()
	if existing, ok := r.summaries[s.ConversationID]; ok {
		s.ID = existing.ID
		s.CreatedAt = existing.CreatedAt
	} else {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	r.summaries[s.ConversationID] = *s
	return nil
}

func copyConversationMessages(in []domain.ConversationMessage) []*domain.ConversationMessage {
	out := make([]*domain.ConversationMessage, 0, len(in))
	for i := range in {
		m := in[i]
		out = append(out, &m)
	}
	return out
}
