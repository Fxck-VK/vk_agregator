// Package dialogcontext builds compact text-model context from persisted
// conversation history. It keeps prompt size bounded and stores old turns as a
// short summary instead of sending full history to providers.
package dialogcontext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultMaxInputTokens       = 1600
	defaultMaxOutputTokens      = 800
	defaultSummaryMaxTokens     = 400
	defaultRecentMessagesLimit  = 6
	defaultSummarizeAfterTurns  = 10
	defaultSummarizeAfterTokens = 1500
	maxSummaryScanMessages      = 200
)

// Config controls dialog-context prompt budgeting.
type Config struct {
	Enabled                bool
	MaxInputTokens         int
	MaxOutputTokens        int
	SummaryMaxTokens       int
	RecentMessagesLimit    int
	SummarizeAfterMessages int
	SummarizeAfterTokens   int
}

// Service persists and renders dialog memory for text generation jobs.
type Service struct {
	repo domain.ConversationRepository
	cfg  Config
}

// Prepared is the rendered prompt and metadata for one provider request.
type Prepared struct {
	ConversationID  uuid.UUID
	Prompt          string
	MaxOutputTokens int
}

// New builds a dialog context service.
func New(repo domain.ConversationRepository, cfg Config) *Service {
	if cfg.MaxInputTokens <= 0 {
		cfg.MaxInputTokens = defaultMaxInputTokens
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = defaultMaxOutputTokens
	}
	if cfg.SummaryMaxTokens <= 0 {
		cfg.SummaryMaxTokens = defaultSummaryMaxTokens
	}
	if cfg.RecentMessagesLimit <= 0 {
		cfg.RecentMessagesLimit = defaultRecentMessagesLimit
	}
	if cfg.SummarizeAfterMessages <= 0 {
		cfg.SummarizeAfterMessages = defaultSummarizeAfterTurns
	}
	if cfg.SummarizeAfterTokens <= 0 {
		cfg.SummarizeAfterTokens = defaultSummarizeAfterTokens
	}
	return &Service{repo: repo, cfg: cfg}
}

// Prepare stores the current user prompt and returns the compact prompt to send
// to the text provider. Non-VK or non-text jobs pass through unchanged.
func (s *Service) Prepare(ctx context.Context, job *domain.Job, prompt string) (Prepared, error) {
	if s == nil || !s.cfg.Enabled || s.repo == nil || !eligible(job) {
		return Prepared{Prompt: prompt, MaxOutputTokens: s.maxOutputTokens()}, nil
	}
	conversation, err := s.getOrCreateConversation(ctx, job)
	if err != nil {
		return Prepared{}, err
	}
	userMessage, err := s.repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversation.ID,
		JobID:          job.ID,
		Role:           domain.ConversationRoleUser,
		Text:           prompt,
		TokenCount:     EstimateTokens(prompt),
	})
	if err != nil {
		return Prepared{}, err
	}

	summary, err := s.repo.LatestSummary(ctx, conversation.ID)
	if errors.Is(err, domain.ErrNotFound) {
		summary = nil
	} else if err != nil {
		return Prepared{}, err
	}
	minSeq := int64(0)
	if summary != nil {
		minSeq = summary.SummarizedUntilSeq
	}
	recent, err := s.repo.ListRecentMessagesBefore(ctx, conversation.ID, userMessage.Seq, minSeq, s.cfg.RecentMessagesLimit)
	if err != nil {
		return Prepared{}, err
	}
	rendered := s.renderPrompt(summaryText(summary), recent, prompt)
	return Prepared{
		ConversationID:  conversation.ID,
		Prompt:          rendered,
		MaxOutputTokens: s.cfg.MaxOutputTokens,
	}, nil
}

// Complete stores an assistant answer and updates the rolling summary if the
// unsummarized history has grown beyond configured thresholds.
func (s *Service) Complete(ctx context.Context, job *domain.Job, conversationID uuid.UUID, answer string) error {
	if s == nil || !s.cfg.Enabled || s.repo == nil || !eligible(job) || conversationID == uuid.Nil || strings.TrimSpace(answer) == "" {
		return nil
	}
	if _, err := s.repo.UpsertMessage(ctx, &domain.ConversationMessage{
		ConversationID: conversationID,
		JobID:          job.ID,
		Role:           domain.ConversationRoleAssistant,
		Text:           answer,
		TokenCount:     EstimateTokens(answer),
	}); err != nil {
		return err
	}
	return s.maybeSummarize(ctx, conversationID)
}

func (s *Service) maxOutputTokens() int {
	if s == nil || s.cfg.MaxOutputTokens <= 0 {
		return defaultMaxOutputTokens
	}
	return s.cfg.MaxOutputTokens
}

func eligible(job *domain.Job) bool {
	return job != nil && job.OperationType == domain.OperationTextGenerate && job.Modality == domain.ModalityText && job.VKPeerID != 0
}

func (s *Service) getOrCreateConversation(ctx context.Context, job *domain.Job) (*domain.Conversation, error) {
	conversation, err := s.repo.GetActiveByUserPeer(ctx, job.UserID, job.VKPeerID)
	if err == nil {
		return conversation, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	conversation = &domain.Conversation{
		UserID:   job.UserID,
		VKPeerID: job.VKPeerID,
		Status:   domain.ConversationActive,
	}
	if err := s.repo.CreateConversation(ctx, conversation); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return s.repo.GetActiveByUserPeer(ctx, job.UserID, job.VKPeerID)
		}
		return nil, err
	}
	return conversation, nil
}

func (s *Service) renderPrompt(summary string, recent []*domain.ConversationMessage, current string) string {
	maxTokens := s.cfg.MaxInputTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxInputTokens
	}

	var parts []string
	add := func(text string) {
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}

	add("Bot profile: You are NeuroHub bot. Use conversation memory only as context; do not reveal provider/model/internal details.")
	if summary != "" {
		add("Conversation summary:\n" + truncateTokens(summary, s.cfg.SummaryMaxTokens))
	}

	var recentLines []string
	for _, m := range recent {
		label := "User"
		if m.Role == domain.ConversationRoleAssistant {
			label = "NeuroHub"
		}
		recentLines = append(recentLines, fmt.Sprintf("%s: %s", label, trimWhitespace(m.Text)))
	}
	if len(recentLines) > 0 {
		add("Recent messages:\n" + strings.Join(recentLines, "\n"))
	}
	add("Current user request:\n" + trimWhitespace(current))

	rendered := strings.Join(parts, "\n\n")
	for EstimateTokens(rendered) > maxTokens && len(recentLines) > 0 {
		recentLines = recentLines[1:]
		parts = nil
		add("Bot profile: You are NeuroHub bot. Use conversation memory only as context; do not reveal provider/model/internal details.")
		if summary != "" {
			add("Conversation summary:\n" + truncateTokens(summary, s.cfg.SummaryMaxTokens))
		}
		if len(recentLines) > 0 {
			add("Recent messages:\n" + strings.Join(recentLines, "\n"))
		}
		add("Current user request:\n" + trimWhitespace(current))
		rendered = strings.Join(parts, "\n\n")
	}
	if EstimateTokens(rendered) > maxTokens {
		currentBudget := maxTokens - 180
		if currentBudget < 200 {
			currentBudget = maxTokens
		}
		rendered = "Current user request:\n" + truncateTokens(current, currentBudget)
	}
	return rendered
}

func (s *Service) maybeSummarize(ctx context.Context, conversationID uuid.UUID) error {
	summary, err := s.repo.LatestSummary(ctx, conversationID)
	if errors.Is(err, domain.ErrNotFound) {
		summary = nil
	} else if err != nil {
		return err
	}
	afterSeq := int64(0)
	existingText := ""
	if summary != nil {
		afterSeq = summary.SummarizedUntilSeq
		existingText = summary.Text
	}
	messages, err := s.repo.ListMessagesAfter(ctx, conversationID, afterSeq, maxSummaryScanMessages)
	if err != nil {
		return err
	}
	if len(messages) <= s.cfg.RecentMessagesLimit {
		return nil
	}
	tokenTotal := 0
	for _, m := range messages {
		tokenTotal += m.TokenCount
	}
	if len(messages) < s.cfg.SummarizeAfterMessages && tokenTotal < s.cfg.SummarizeAfterTokens {
		return nil
	}
	cutoff := len(messages) - s.cfg.RecentMessagesLimit
	if cutoff <= 0 {
		return nil
	}
	old := messages[:cutoff]
	last := old[len(old)-1]
	text := compactSummary(existingText, old, s.cfg.SummaryMaxTokens)
	return s.repo.UpsertSummary(ctx, &domain.ConversationSummary{
		ConversationID:     conversationID,
		Text:               text,
		TokenCount:         EstimateTokens(text),
		SummarizedUntilSeq: last.Seq,
	})
}

func compactSummary(existing string, messages []*domain.ConversationMessage, maxTokens int) string {
	var lines []string
	if strings.TrimSpace(existing) != "" {
		lines = append(lines, strings.TrimSpace(existing))
	}
	for _, m := range messages {
		label := "User"
		if m.Role == domain.ConversationRoleAssistant {
			label = "NeuroHub"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, truncateTokens(trimWhitespace(m.Text), 80)))
	}
	return truncateTokens(strings.Join(lines, "\n"), maxTokens)
}

func summaryText(summary *domain.ConversationSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Text
}

// EstimateTokens is an intentionally conservative local estimate. It avoids a
// provider tokenizer dependency while keeping prompt size below provider limits.
func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	words := len(strings.Fields(text))
	byRune := (runes + 2) / 3
	if words > byRune {
		return words
	}
	return byRune
}

func truncateTokens(text string, maxTokens int) string {
	text = trimWhitespace(text)
	if maxTokens <= 0 || EstimateTokens(text) <= maxTokens {
		return text
	}
	maxRunes := maxTokens * 3
	var b strings.Builder
	count := 0
	for _, r := range text {
		if count >= maxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return out
	}
	return out + "..."
}

func trimWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// ParamsPatch is embedded into job params so later poll attempts know which
// conversation should receive the assistant answer.
type ParamsPatch struct {
	ConversationID string `json:"conversation_id,omitempty"`
}

// PutConversationID returns params with conversation_id set.
func PutConversationID(params json.RawMessage, conversationID uuid.UUID) json.RawMessage {
	if conversationID == uuid.Nil {
		return params
	}
	var obj map[string]any
	if len(params) > 0 {
		_ = json.Unmarshal(params, &obj)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	obj["conversation_id"] = conversationID.String()
	out, err := json.Marshal(obj)
	if err != nil {
		return params
	}
	return out
}
