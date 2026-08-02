// Package conversationtitle creates concise, best-effort titles for new Web
// conversations. It is deliberately separate from user-job processing: title
// generation never changes a Job, billing record, artifact, or delivery.
package conversationtitle

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
)

const (
	maxTitleRunes           = 80
	maxPromptRunes          = 1200
	defaultCallTimeout      = 15 * time.Second
	defaultFirstMessageWait = 500 * time.Millisecond
	// Five bounded indexed lookups at most under the default 500 ms race window
	// keep the rare ordering recovery cheap at high request volume.
	defaultFirstMessagePollInterval = 100 * time.Millisecond
)

// ErrFirstUserMessageNotReady is retryable only while the normal text worker
// may still be persisting the first user message for a queued Web job.
var ErrFirstUserMessageNotReady = errors.New("conversationtitle: first user message not ready")

// Generator is the narrow provider boundary for background title creation. It
// intentionally has no Job, pricing, or delivery data in its request.
type Generator interface {
	GenerateConversationTitle(ctx context.Context, conversationID uuid.UUID, prompt string) (string, error)
}

// Deps supplies the persistence and provider boundaries for Service.
type Deps struct {
	Jobs                     domain.JobRepository
	Conversations            domain.ConversationRepository
	Generator                Generator
	CallTimeout              time.Duration
	FirstMessageWait         time.Duration
	FirstMessagePollInterval time.Duration
}

// Service processes one isolated title-stream task at a time.
type Service struct {
	jobs                     domain.JobRepository
	conversations            domain.ConversationRepository
	generator                Generator
	callTimeout              time.Duration
	firstMessageWait         time.Duration
	firstMessagePollInterval time.Duration
}

// New creates a title service. A nil generator is intentional in deployments
// without DeepInfra: title tasks are acknowledged as no-ops and the fallback
// remains available to the user.
func New(deps Deps) *Service {
	timeout := deps.CallTimeout
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}
	firstMessageWait := deps.FirstMessageWait
	if firstMessageWait <= 0 {
		firstMessageWait = defaultFirstMessageWait
	}
	firstMessagePollInterval := deps.FirstMessagePollInterval
	if firstMessagePollInterval <= 0 {
		firstMessagePollInterval = defaultFirstMessagePollInterval
	}
	if firstMessagePollInterval > firstMessageWait {
		firstMessagePollInterval = firstMessageWait
	}
	return &Service{
		jobs:                     deps.Jobs,
		conversations:            deps.Conversations,
		generator:                deps.Generator,
		callTimeout:              timeout,
		firstMessageWait:         firstMessageWait,
		firstMessagePollInterval: firstMessagePollInterval,
	}
}

// Process returns nil when this title task can be acknowledged. Provider and
// malformed-output failures are deliberately successful no-ops. It waits only
// briefly for the normal worker's first-message persistence race; a remaining
// miss is retried through the title stream's durable recovery path.
func (s *Service) Process(ctx context.Context, task queue.Task) error {
	if s == nil || s.generator == nil {
		return nil
	}
	if s.jobs == nil || s.conversations == nil {
		return errors.New("conversationtitle: required dependency unavailable")
	}
	job, err := s.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !eligibleWebTextJob(job) {
		return nil
	}

	message, err := s.firstUserMessage(ctx, job)
	if err != nil {
		return err
	}
	if message == nil || message.ConversationID == uuid.Nil || message.JobID != job.ID {
		return nil
	}

	conversation, err := s.conversations.GetByIDForAccount(ctx, job.AccountID, message.ConversationID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !eligibleConversation(conversation, job.AccountID) {
		return nil
	}

	first, err := s.conversations.GetFirstUserMessage(ctx, conversation.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if first == nil || first.ConversationID != conversation.ID || first.JobID != job.ID {
		return nil
	}

	fallback := FallbackTitle(message.Text)
	if fallback == "" {
		return nil
	}
	changedFallback, err := s.conversations.SetConversationFallbackTitleIfPending(ctx, conversation.ID, fallback)
	if err != nil {
		return err
	}
	if !changedFallback {
		// The normal worker may have recorded the fallback after our initial
		// read. Revalidate only on this rare CAS miss so that a valid fallback
		// race still gets a semantic title, while manual/generated titles stop.
		conversation, err = s.conversations.GetByIDForAccount(ctx, job.AccountID, conversation.ID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if !eligibleConversation(conversation, job.AccountID) {
			return nil
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout)
	title, err := s.generator.GenerateConversationTitle(callCtx, conversation.ID, boundedPrompt(message.Text))
	cancel()
	if err != nil {
		return nil
	}
	title = NormalizeTitle(title)
	if title == "" {
		return nil
	}
	_, err = s.conversations.SetGeneratedTitleForActiveWebConversation(ctx, job.AccountID, conversation.ID, title)
	return err
}

// firstUserMessage absorbs the normal ordering race without exposing it to the
// browser or holding the user-generation worker. This method runs only in the
// isolated title worker and makes a bounded number of indexed job-ID lookups.
func (s *Service) firstUserMessage(ctx context.Context, job *domain.Job) (*domain.ConversationMessage, error) {
	message, err := s.conversations.GetUserMessageByJobID(ctx, job.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		return message, err
	}
	if !mayStillPersistFirstMessage(job.Status) {
		return nil, nil
	}

	timer := time.NewTimer(s.firstMessageWait)
	defer timer.Stop()
	ticker := time.NewTicker(s.firstMessagePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			// A final lookup closes the timer/ticker boundary: the normal worker
			// may have committed just before the timer became selectable.
			message, err = s.conversations.GetUserMessageByJobID(ctx, job.ID)
			if errors.Is(err, domain.ErrNotFound) {
				return nil, ErrFirstUserMessageNotReady
			}
			return message, err
		case <-ticker.C:
			message, err = s.conversations.GetUserMessageByJobID(ctx, job.ID)
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return message, err
		}
	}
}

func eligibleWebTextJob(job *domain.Job) bool {
	return job != nil &&
		job.AccountID != uuid.Nil &&
		job.Source == "web" &&
		job.OperationType == domain.OperationTextGenerate &&
		job.Modality == domain.ModalityText
}

func eligibleConversation(conversation *domain.Conversation, accountID uuid.UUID) bool {
	if conversation == nil || conversation.AccountID != accountID ||
		conversation.Source != domain.ConversationSourceWeb ||
		conversation.Status != domain.ConversationActive {
		return false
	}
	switch conversation.TitleOrigin {
	case domain.ConversationTitleOriginAutoPending, domain.ConversationTitleOriginAutoFallback:
		return true
	default:
		return false
	}
}

func mayStillPersistFirstMessage(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusQueued, domain.JobStatusDispatchingProvider:
		return true
	default:
		return false
	}
}

// FallbackTitle matches the deterministic first-prompt title written by the
// normal dialog-context worker. It never sends any content to a provider.
func FallbackTitle(prompt string) string {
	return truncateRunes(strings.Join(strings.Fields(prompt), " "), maxTitleRunes, true)
}

// NormalizeTitle validates provider output before it reaches durable state.
// It permits one compact line only, removes wrapping quotes and caps Unicode
// runes without splitting a code point.
func NormalizeTitle(raw string) string {
	title := strings.Join(strings.Fields(raw), " ")
	for {
		stripped, ok := stripWrappingQuotes(title)
		if !ok {
			break
		}
		title = strings.Join(strings.Fields(stripped), " ")
	}
	return truncateRunes(title, maxTitleRunes, false)
}

func boundedPrompt(prompt string) string {
	return truncateRunes(strings.Join(strings.Fields(prompt), " "), maxPromptRunes, false)
}

func stripWrappingQuotes(value string) (string, bool) {
	if utf8.RuneCountInString(value) < 2 {
		return value, false
	}
	pairs := map[rune]rune{'"': '"', '\'': '\'', '«': '»', '“': '”', '„': '“'}
	runes := []rune(value)
	if close, ok := pairs[runes[0]]; ok && runes[len(runes)-1] == close {
		return string(runes[1 : len(runes)-1]), true
	}
	return value, false
}

func truncateRunes(value string, max int, ellipsis bool) string {
	value = strings.TrimSpace(value)
	if value == "" || max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if ellipsis && max > 3 {
		return strings.TrimSpace(string(runes[:max-3])) + "..."
	}
	return string(runes[:max])
}
