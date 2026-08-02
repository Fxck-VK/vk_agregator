package conversationtitle_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/conversationtitle"
)

type generatorStub struct {
	result string
	err    error
	calls  int
	prompt string
	id     uuid.UUID
}

type fallbackRaceRepo struct {
	domain.ConversationRepository
	injectFallback bool
}

type firstMessageGateRepo struct {
	domain.ConversationRepository
	firstMissed chan struct{}
	once        sync.Once
}

func (r *firstMessageGateRepo) GetUserMessageByJobID(ctx context.Context, jobID uuid.UUID) (*domain.ConversationMessage, error) {
	message, err := r.ConversationRepository.GetUserMessageByJobID(ctx, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		r.once.Do(func() { close(r.firstMissed) })
	}
	return message, err
}

func (r *fallbackRaceRepo) SetConversationFallbackTitleIfPending(ctx context.Context, conversationID uuid.UUID, title string) (bool, error) {
	if r.injectFallback {
		r.injectFallback = false
		if _, err := r.ConversationRepository.SetConversationFallbackTitleIfPending(ctx, conversationID, "Concurrent fallback"); err != nil {
			return false, err
		}
		return false, nil
	}
	return r.ConversationRepository.SetConversationFallbackTitleIfPending(ctx, conversationID, title)
}

func (g *generatorStub) GenerateConversationTitle(_ context.Context, conversationID uuid.UUID, prompt string) (string, error) {
	g.calls++
	g.id = conversationID
	g.prompt = prompt
	return g.result, g.err
}

type titleFixture struct {
	accountID     uuid.UUID
	conversation  *domain.Conversation
	job           *domain.Job
	jobs          *memory.JobRepo
	conversations *memory.ConversationRepo
	generator     *generatorStub
	service       *conversationtitle.Service
}

func newTitleFixture(t *testing.T) *titleFixture {
	t.Helper()
	ctx := context.Background()
	accountID := uuid.New()
	conversations := memory.NewConversationRepo()
	conversation := &domain.Conversation{
		AccountID:   accountID,
		Source:      domain.ConversationSourceWeb,
		Status:      domain.ConversationActive,
		TitleOrigin: domain.ConversationTitleOriginAutoPending,
	}
	if err := conversations.CreateConversation(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	jobs := memory.NewJobRepo()
	job := newWebTextJob(accountID)
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	generator := &generatorStub{result: "Semantic title"}
	return &titleFixture{
		accountID:     accountID,
		conversation:  conversation,
		job:           job,
		jobs:          jobs,
		conversations: conversations,
		generator:     generator,
		service: conversationtitle.New(conversationtitle.Deps{
			Jobs:          jobs,
			Conversations: conversations,
			Generator:     generator,
		}),
	}
}

func newWebTextJob(accountID uuid.UUID) *domain.Job {
	return &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		Source:         "web",
		ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:     domain.ResultModeAccountHistory,
		OperationType:  domain.OperationTextGenerate,
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusQueued,
		IdempotencyKey: "title-test:" + uuid.NewString(),
		CorrelationID:  "title-test",
		CostEstimate:   17,
		CostReserved:   17,
		CostCaptured:   0,
	}
}

func titleTask(job *domain.Job) queue.Task {
	return queue.Task{
		JobID:         job.ID,
		Operation:     domain.OperationTextGenerate,
		Modality:      domain.ModalityText,
		CorrelationID: job.CorrelationID,
	}
}

func addUserMessage(t *testing.T, repo *memory.ConversationRepo, conversationID, jobID uuid.UUID, text string) {
	t.Helper()
	if _, err := repo.UpsertMessage(context.Background(), &domain.ConversationMessage{
		ConversationID: conversationID,
		JobID:          jobID,
		Role:           domain.ConversationRoleUser,
		Text:           text,
	}); err != nil {
		t.Fatalf("store user message: %v", err)
	}
}

func TestProcessWaitsForFirstMessageThenGeneratesOnce(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)

	err := fixture.service.Process(ctx, titleTask(fixture.job))
	if !errors.Is(err, conversationtitle.ErrFirstUserMessageNotReady) {
		t.Fatalf("Process() error = %v, want ErrFirstUserMessageNotReady", err)
	}
	if fixture.generator.calls != 0 {
		t.Fatalf("generator calls before persisted prompt = %d, want 0", fixture.generator.calls)
	}

	const prompt = "Make a detailed launch plan for a multilingual photo editor"
	addUserMessage(t, fixture.conversations, fixture.conversation.ID, fixture.job.ID, prompt)
	if err := fixture.service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() after persisted prompt: %v", err)
	}
	if fixture.generator.calls != 1 || fixture.generator.id != fixture.conversation.ID || fixture.generator.prompt != prompt {
		t.Fatalf("unexpected generator invocation: %+v", fixture.generator)
	}
	stored, err := fixture.conversations.GetByIDForAccount(ctx, fixture.accountID, fixture.conversation.ID)
	if err != nil {
		t.Fatalf("load generated conversation: %v", err)
	}
	if stored.Title != "Semantic title" || stored.TitleOrigin != domain.ConversationTitleOriginAutoGenerated {
		t.Fatalf("generated conversation = %+v", stored)
	}
	before, err := fixture.jobs.GetByID(ctx, fixture.job.ID)
	if err != nil {
		t.Fatalf("load original job: %v", err)
	}
	if err := fixture.service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() retry: %v", err)
	}
	if fixture.generator.calls != 1 {
		t.Fatalf("generator calls after duplicate task = %d, want 1", fixture.generator.calls)
	}
	after, err := fixture.jobs.GetByID(ctx, fixture.job.ID)
	if err != nil {
		t.Fatalf("load job after title work: %v", err)
	}
	if after.Status != before.Status || after.CostEstimate != before.CostEstimate || after.CostReserved != before.CostReserved || after.CostCaptured != before.CostCaptured {
		t.Fatalf("title work mutated user job: before=%+v after=%+v", before, after)
	}
}

func TestProcessWaitsBrieflyForMessagePersistedByNormalWorker(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	firstMissed := make(chan struct{})
	conversations := &firstMessageGateRepo{
		ConversationRepository: fixture.conversations,
		firstMissed:            firstMissed,
	}
	service := conversationtitle.New(conversationtitle.Deps{
		Jobs:                     fixture.jobs,
		Conversations:            conversations,
		Generator:                fixture.generator,
		FirstMessageWait:         100 * time.Millisecond,
		FirstMessagePollInterval: 5 * time.Millisecond,
	})
	persisted := make(chan error, 1)
	go func() {
		<-firstMissed
		_, err := fixture.conversations.UpsertMessage(ctx, &domain.ConversationMessage{
			ConversationID: fixture.conversation.ID,
			JobID:          fixture.job.ID,
			Role:           domain.ConversationRoleUser,
			Text:           "Give this new chat a semantic title",
		})
		persisted <- err
	}()

	if err := service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() = %v, want the normal-worker race to resolve in-handler", err)
	}
	if err := <-persisted; err != nil {
		t.Fatalf("persist first message: %v", err)
	}
	if fixture.generator.calls != 1 {
		t.Fatalf("generator calls = %d, want 1 after the first-message race", fixture.generator.calls)
	}
}

func TestProcessIgnoresLaterMessageAndManualRename(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	firstJob := fixture.job
	secondJob := newWebTextJob(fixture.accountID)
	if err := fixture.jobs.Create(ctx, secondJob); err != nil {
		t.Fatalf("create later job: %v", err)
	}
	addUserMessage(t, fixture.conversations, fixture.conversation.ID, firstJob.ID, "First topic")
	addUserMessage(t, fixture.conversations, fixture.conversation.ID, secondJob.ID, "Later topic")
	if _, err := fixture.conversations.RenameActiveConversationForAccount(ctx, fixture.accountID, fixture.conversation.ID, domain.ConversationSourceWeb, "Manual title"); err != nil {
		t.Fatalf("manual rename: %v", err)
	}

	if err := fixture.service.Process(ctx, titleTask(secondJob)); err != nil {
		t.Fatalf("Process() later job: %v", err)
	}
	if err := fixture.service.Process(ctx, titleTask(firstJob)); err != nil {
		t.Fatalf("Process() manually renamed first job: %v", err)
	}
	if fixture.generator.calls != 0 {
		t.Fatalf("generator calls = %d, want 0 after later/manual messages", fixture.generator.calls)
	}
	stored, err := fixture.conversations.GetByIDForAccount(ctx, fixture.accountID, fixture.conversation.ID)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if stored.Title != "Manual title" || stored.TitleOrigin != domain.ConversationTitleOriginManual {
		t.Fatalf("manual title was overwritten: %+v", stored)
	}
}

func TestProcessGeneratesWhenFallbackWinsParallelRace(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	const prompt = "Create a concise title even when fallback races"
	addUserMessage(t, fixture.conversations, fixture.conversation.ID, fixture.job.ID, prompt)
	racingRepo := &fallbackRaceRepo{ConversationRepository: fixture.conversations, injectFallback: true}
	service := conversationtitle.New(conversationtitle.Deps{
		Jobs:          fixture.jobs,
		Conversations: racingRepo,
		Generator:     fixture.generator,
	})

	if err := service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() = %v", err)
	}
	if fixture.generator.calls != 1 {
		t.Fatalf("generator calls = %d, want 1 after fallback race", fixture.generator.calls)
	}
	stored, err := fixture.conversations.GetByIDForAccount(ctx, fixture.accountID, fixture.conversation.ID)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if stored.Title != "Semantic title" || stored.TitleOrigin != domain.ConversationTitleOriginAutoGenerated {
		t.Fatalf("semantic title was lost after fallback race: %+v", stored)
	}
}

func TestProcessLeavesFallbackOnProviderOrMalformedOutput(t *testing.T) {
	tests := []struct {
		name   string
		result string
		err    error
	}{
		{name: "provider failure", err: errors.New("provider unavailable")},
		{name: "only wrapping quotes", result: "  \"\"  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := newTitleFixture(t)
			fixture.generator.result = tt.result
			fixture.generator.err = tt.err
			const prompt = "Fallback remains if the title model is unavailable"
			addUserMessage(t, fixture.conversations, fixture.conversation.ID, fixture.job.ID, prompt)
			before, err := fixture.jobs.GetByID(ctx, fixture.job.ID)
			if err != nil {
				t.Fatalf("load job before title work: %v", err)
			}

			if err := fixture.service.Process(ctx, titleTask(fixture.job)); err != nil {
				t.Fatalf("Process() = %v, want successful no-op", err)
			}
			stored, err := fixture.conversations.GetByIDForAccount(ctx, fixture.accountID, fixture.conversation.ID)
			if err != nil {
				t.Fatalf("load conversation: %v", err)
			}
			if stored.Title != prompt || stored.TitleOrigin != domain.ConversationTitleOriginAutoFallback {
				t.Fatalf("fallback was not preserved: %+v", stored)
			}
			after, err := fixture.jobs.GetByID(ctx, fixture.job.ID)
			if err != nil {
				t.Fatalf("load job after title work: %v", err)
			}
			if after.Status != before.Status || after.CostEstimate != before.CostEstimate || after.CostReserved != before.CostReserved || after.CostCaptured != before.CostCaptured {
				t.Fatalf("title work mutated user job: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestProcessIgnoresNonWebJobs(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	fixture.job.Source = "miniapp"
	if err := fixture.jobs.Update(ctx, fixture.job); err != nil {
		t.Fatalf("persist non-web job: %v", err)
	}
	addUserMessage(t, fixture.conversations, fixture.conversation.ID, fixture.job.ID, "This must not reach DeepSeek")

	if err := fixture.service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() = %v", err)
	}
	if fixture.generator.calls != 0 {
		t.Fatalf("non-web job called generator %d times", fixture.generator.calls)
	}
}

func TestProcessAcknowledgesWhenGeneratorIsUnavailable(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	serviceWithoutGenerator := conversationtitle.New(conversationtitle.Deps{
		Jobs:          fixture.jobs,
		Conversations: fixture.conversations,
	})

	if err := serviceWithoutGenerator.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() without generator = %v, want acknowledged no-op", err)
	}
}

func TestProcessAcknowledgesMissingPromptAfterShortRaceWindow(t *testing.T) {
	ctx := context.Background()
	fixture := newTitleFixture(t)
	if err := fixture.jobs.UpdateStatus(ctx, fixture.job.ID, domain.JobStatusQueued, domain.JobStatusDispatchingProvider, "", ""); err != nil {
		t.Fatalf("mark job dispatching: %v", err)
	}
	if err := fixture.jobs.UpdateStatus(ctx, fixture.job.ID, domain.JobStatusDispatchingProvider, domain.JobStatusProviderSubmitted, "", ""); err != nil {
		t.Fatalf("mark job submitted: %v", err)
	}

	if err := fixture.service.Process(ctx, titleTask(fixture.job)); err != nil {
		t.Fatalf("Process() after first-message race = %v, want acknowledged no-op", err)
	}
	if fixture.generator.calls != 0 {
		t.Fatalf("generator calls = %d, want 0 without persisted first prompt", fixture.generator.calls)
	}
}

func TestNormalizeTitleCollapsesQuotesAndCapsRunes(t *testing.T) {
	long := strings.Repeat("Ж", 100)
	got := conversationtitle.NormalizeTitle("  \"  first\n second  " + long + "  \"  ")
	if strings.Contains(got, "\n") || strings.HasPrefix(got, "\"") || strings.HasSuffix(got, "\"") {
		t.Fatalf("title was not normalized: %q", got)
	}
	if utf8.RuneCountInString(got) > 80 {
		t.Fatalf("title rune count = %d, want <= 80", utf8.RuneCountInString(got))
	}
	if conversationtitle.NormalizeTitle("  \"\"  ") != "" {
		t.Fatal("empty quoted title must be rejected")
	}
}
