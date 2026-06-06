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
	activeByRef    map[string]uuid.UUID
	messagesByID   map[uuid.UUID]domain.ConversationMessage
	messageByRole  map[string]uuid.UUID
	byConversation map[uuid.UUID][]uuid.UUID
	summaries      map[uuid.UUID]domain.ConversationSummary
}

// NewConversationRepo builds an empty ConversationRepo.
func NewConversationRepo() *ConversationRepo {
	return &ConversationRepo{
		byID:           map[uuid.UUID]domain.Conversation{},
		activeByRef:    map[string]uuid.UUID{},
		messagesByID:   map[uuid.UUID]domain.ConversationMessage{},
		messageByRole:  map[string]uuid.UUID{},
		byConversation: map[uuid.UUID][]uuid.UUID{},
		summaries:      map[uuid.UUID]domain.ConversationSummary{},
	}
}

var _ domain.ConversationRepository = (*ConversationRepo)(nil)

func activeConversationKey(userID uuid.UUID, peerID int64) string {
	return activeConversationRefKey(domain.ConversationRef{
		UserID:   userID,
		Source:   domain.ConversationSourceVKBot,
		VKPeerID: peerID,
	})
}

func activeConversationRefKey(ref domain.ConversationRef) string {
	source := ref.Source
	if source == "" {
		source = domain.ConversationSourceVKBot
	}
	if source == domain.ConversationSourceVKBot {
		return ref.UserID.String() + "|" + string(source) + "|" + strconv.FormatInt(ref.VKPeerID, 10)
	}
	return ref.UserID.String() + "|" + string(source) + "|" + ref.ExternalThreadID
}

func messageRoleKey(jobID uuid.UUID, role domain.ConversationMessageRole) string {
	return jobID.String() + "|" + string(role)
}

func (r *ConversationRepo) GetActiveByUserPeer(_ context.Context, userID uuid.UUID, vkPeerID int64) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.activeByRef[activeConversationKey(userID, vkPeerID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := r.byID[id]
	return &c, nil
}

func (r *ConversationRepo) GetActiveByReference(_ context.Context, ref domain.ConversationRef) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.activeByRef[activeConversationRefKey(ref)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := r.byID[id]
	return &c, nil
}

func (r *ConversationRepo) GetByIDForUser(_ context.Context, userID, conversationID uuid.UUID) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[conversationID]
	if !ok || c.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return &c, nil
}

func (r *ConversationRepo) ListByUserSource(_ context.Context, userID uuid.UUID, source domain.ConversationSource, limit, offset int) ([]*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		return nil, nil
	}
	if offset < 0 {
		offset = 0
	}
	var matched []domain.Conversation
	for _, c := range r.byID {
		if c.UserID == userID && c.Source == source {
			matched = append(matched, c)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	if offset > len(matched) {
		return nil, nil
	}
	matched = matched[offset:]
	if len(matched) > limit {
		matched = matched[:limit]
	}
	out := make([]*domain.Conversation, 0, len(matched))
	for i := range matched {
		c := matched[i]
		out = append(out, &c)
	}
	return out, nil
}

func (r *ConversationRepo) CreateConversation(_ context.Context, c *domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.Source == "" {
		c.Source = domain.ConversationSourceVKBot
	}
	if c.Status == "" {
		c.Status = domain.ConversationActive
	}
	key := activeConversationRefKey(domain.ConversationRef{
		UserID:           c.UserID,
		Source:           c.Source,
		VKPeerID:         c.VKPeerID,
		ExternalThreadID: c.ExternalThreadID,
	})
	if c.Status == domain.ConversationActive {
		if _, ok := r.activeByRef[key]; ok {
			return domain.ErrConflict
		}
		r.activeByRef[key] = c.ID
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
