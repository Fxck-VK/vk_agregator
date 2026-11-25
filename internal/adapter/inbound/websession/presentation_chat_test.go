package websession

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/presentationintake"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/resultservice"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func TestPresentationChatQuoteAndSeparateIntakeJob(t *testing.T) {
	h, _, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	h.deps.ImagePricing = prices
	h.cfg.Environment = "development"
	h.cfg.TextModels = textgeneration.Models([]string{providermodels.PublicTextGemini37Flash}, prices)
	owner := uuid.New()
	routes := h.Routes()
	quoteRequest := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/presentation-chats/quote", sessions, owner)
	quoteRequest.Header.Set("X-Epifera-Release-View", "dev")
	quoted := httptest.NewRecorder()
	routes.ServeHTTP(quoted, quoteRequest)
	var quote struct {
		Model string `json:"model_id"`
		Deck  int64  `json:"credits"`
		Turn  int64  `json:"turn_credits"`
	}
	if quoted.Code != http.StatusOK || json.Unmarshal(quoted.Body.Bytes(), &quote) != nil || quote.Deck <= 0 || quote.Turn <= 0 || quote.Model != providermodels.PublicTextGemini37Flash {
		t.Fatalf("quote: %d %s", quoted.Code, quoted.Body)
	}
	key := uuid.New()
	chat := uuid.Nil
	for range 2 {
		create := safeWebConversationMessageRequest(t, sessions, owner, uuid.New(), key, `{}`)
		create.URL.Path = "/web/v1/presentation-chats"
		create.Header.Set("X-Epifera-Release-View", "dev")
		created := httptest.NewRecorder()
		routes.ServeHTTP(created, create)
		var result struct {
			ID   uuid.UUID `json:"id"`
			Kind string    `json:"kind"`
		}
		if created.Code != http.StatusCreated && created.Code != http.StatusOK || json.Unmarshal(created.Body.Bytes(), &result) != nil || result.ID == uuid.Nil || result.Kind != "presentation" {
			t.Fatalf("create: %d %s", created.Code, created.Body)
		}
		if chat != uuid.Nil && chat != result.ID {
			t.Fatal("conversation replay returned a new conversation")
		}
		chat = result.ID
	}
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	turn := func(account uuid.UUID, expected int64, mode string) int {
		body := fmt.Sprintf(`{"prompt":"Тема ещё не выбрана","model_id":%q,"expected_presentation_credits":%d,"expected_turn_credits":%d}`, quote.Model, quote.Deck, expected)
		req := safeWebConversationMessageRequest(t, sessions, account, chat, uuid.New(), body)
		req.URL.Path = "/web/v1/presentation-chats/" + chat.String() + "/messages"
		req.Header.Set("X-Epifera-Release-View", mode)
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)
		return rec.Code
	}
	if turn(owner, quote.Turn+1, "dev") != http.StatusConflict || turn(uuid.New(), quote.Turn, "dev") != http.StatusNotFound || turn(owner, quote.Turn, "prod") != http.StatusNotFound || len(jobs.inputs) != 0 {
		t.Fatal("intake accepted an unowned conversation, stale price or PROD request")
	}
	if status := turn(owner, quote.Turn, "dev"); status != http.StatusCreated || len(jobs.inputs) != 1 {
		t.Fatalf("valid intake failed: status=%d jobs=%d", status, len(jobs.inputs))
	}
	if in := jobs.inputs[0]; !strings.HasPrefix(in.IdempotencyKey, "web-presentation-intake:"+owner.String()+":") || in.PricingSnapshot.InternalCredits != quote.Turn || in.ConversationTitleRequested {
		t.Fatalf("intake did not use separate metering and marker: %+v", in)
	}
}

func TestPresentationChatOnlyLaunchesCompleteEvidenceAndRecoversOneDeck(t *testing.T) {
	for _, tc := range []struct {
		name       string
		user       string
		result     string
		wantStatus int
		staleQuote bool
	}{
		{
			name:       "unsupported extracted count",
			user:       "Тема Керамика. Цель обучение. Аудитория студенты. Стиль деловой. Тезисы история и техники.",
			result:     `{"topic":"Керамика","slide_count":10,"count_evidence":"10 слайдов","purpose":"обучение","audience":"студенты","style":"деловой","key_points":"история и техники"}`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "malformed provider payload",
			user:       "Керамика на 10 слайдов",
			result:     `{"raw_private":"formatter failure"}`,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "complete evidence",
			user:       "Тема Керамика. Сделай 10 слайдов. Цель обучение. Аудитория студенты. Стиль деловой. Тезисы история и техники. Музейный каталог.",
			result:     `{"topic":"Керамика","slide_count":10,"count_evidence":"10 слайдов","purpose":"обучение","audience":"студенты","style":"деловой","key_points":"история и техники","notes":"Музейный каталог"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "explicitly refreshed quote",
			user:       "Тема науки. Сделай 5 слайдов. Цель обучение. Аудитория школьники. Стиль минимализм. Тезисы три открытия.",
			result:     `{"topic":"науки","slide_count":5,"count_evidence":"5 слайдов","purpose":"обучение","audience":"школьники","style":"минимализм","key_points":"три открытия"}`,
			wantStatus: http.StatusCreated,
			staleQuote: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
			prices, err := pricingcatalog.NewStaticCatalog()
			if err != nil {
				t.Fatal(err)
			}
			h.deps.ImagePricing = prices
			h.cfg.Environment = "development"
			modelID := providermodels.PublicTextGemini37Flash
			h.cfg.TextModels = textgeneration.Models([]string{modelID}, prices)
			model, turnPrice, err := textgeneration.Resolve(modelID, tc.user, h.cfg.TextModels, prices)
			if err != nil {
				t.Fatal(err)
			}
			deckPrice, err := pricingcatalog.PresentationTextSnapshot(turnPrice)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			owner, turnID, artifactID := uuid.New(), uuid.New(), uuid.New()
			chat, _, err := h.ensurePresentationConversation(ctx, owner, domain.WebPresentationChatReference(uuid.New()), "Epifera Презентации")
			if err != nil {
				t.Fatal(err)
			}
			params := newPresentationChatJobParams(chat.ID, tc.user, model)
			params.PresentationCredits = deckPrice.InternalCredits
			if tc.staleQuote {
				params.PresentationCredits++
			}
			raw, _ := json.Marshal(params)
			price, _ := json.Marshal(turnPrice)
			key := domain.WebPresentationIntakeJobKeyPrefix(owner) + uuid.NewString()
			job := newPersistedWebChatJob(joborchestrator.CreateJobInput{
				AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
				ResultMode: domain.ResultModeAccountHistory, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText,
				IdempotencyKey: key, CorrelationID: key, Params: raw,
			}, turnID, domain.JobStatusSucceeded)
			job.PricingSnapshot = price
			job.OutputArtifactIDs = []uuid.UUID{artifactID}
			if _, err := conversations.UpsertMessage(ctx, &domain.ConversationMessage{ConversationID: chat.ID, JobID: turnID, Role: domain.ConversationRoleUser, Text: tc.user}); err != nil {
				t.Fatal(err)
			}
			intake, _ := presentationintake.Parse(tc.result, []string{tc.user})
			if _, err := conversations.UpsertMessage(ctx, &domain.ConversationMessage{ConversationID: chat.ID, JobID: turnID, Role: domain.ConversationRoleAssistant, Text: intake.Reply}); err != nil {
				t.Fatal(err)
			}
			metadata := resultservice.ArtifactMetadata{ID: artifactID, MediaType: domain.MediaTypeText, MIMEType: presentationAnswerMIME, SizeBytes: int64(len(tc.result))}
			artifact := &imageArtifactReaderStub{artifact: &domain.Artifact{
				ID: artifactID, JobID: &turnID, OwnerAccountID: owner, Status: domain.ArtifactStatusReady, Kind: domain.ArtifactKindOutput,
				MediaType: metadata.MediaType, MimeType: metadata.MIMEType, SizeBytes: metadata.SizeBytes,
				SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(tc.result))), StorageBucket: "fixture", StorageKey: "intake.txt",
			}}
			store := &boundedPresentationStore{data: []byte(tc.result)}
			jobs := &presentationChatTurnJobsStub{job: job, decks: map[string]*domain.Job{}}
			h.deps.ImageJobIdempotency, h.deps.ImageJobReader, h.deps.PresentationChatTurns, h.deps.WebChatJobs = jobs, jobs, jobs, jobs
			h.deps.ImageJobs = jobs
			h.deps.ImageArtifactURLSigner, h.deps.ImageArtifacts = store, artifact
			results := &imageResultReaderStub{result: resultservice.Result{
				ID: turnID, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText,
				Status: domain.JobStatusSucceeded, Artifacts: []resultservice.ArtifactMetadata{metadata},
			}}
			h.deps.ImageResults = results
			routes := h.Routes()
			base := "/web/v1/presentation-chats/" + chat.ID.String() + "/turns/" + turnID.String()
			read := func() *httptest.ResponseRecorder {
				req := authenticatedConversationRequest(t, http.MethodGet, base, sessions, owner)
				req.Header.Set("X-Epifera-Release-View", "dev")
				rec := httptest.NewRecorder()
				routes.ServeHTTP(rec, req)
				return rec
			}
			generate := func(credits int64, updatedQuote bool) *httptest.ResponseRecorder {
				body := fmt.Sprintf(`{"model_id":%q,"expected_presentation_credits":%d,"accept_updated_quote":%t}`, modelID, credits, updatedQuote)
				req := safeWebConversationMessageRequest(t, sessions, owner, chat.ID, uuid.New(), body)
				req.URL.Path = base + "/generate"
				req.Header.Set("X-Epifera-Release-View", "dev")
				rec := httptest.NewRecorder()
				routes.ServeHTTP(rec, req)
				return rec
			}
			before := read()
			if strings.Contains(before.Body.String(), "count_evidence") || strings.Contains(before.Body.String(), "raw_private") {
				t.Fatal("raw extraction leaked to browser")
			}
			if tc.wantStatus != http.StatusServiceUnavailable && !strings.Contains(before.Body.String(), `"missing":`) {
				t.Fatalf("safe projection missing its progress: %d %s", before.Code, before.Body)
			}
			if tc.staleQuote {
				for _, credits := range []int64{params.PresentationCredits, deckPrice.InternalCredits} {
					if blocked := generate(credits, false); blocked.Code != http.StatusConflict || len(jobs.decks) != 0 {
						t.Fatalf("stale or unconfirmed quote started a deck: %d %s", blocked.Code, blocked.Body)
					}
				}
			}
			got := generate(deckPrice.InternalCredits, tc.staleQuote)
			if got.Code != tc.wantStatus {
				t.Fatalf("finalize = %d %s, want %d", got.Code, got.Body, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusCreated {
				if len(jobs.decks) != 0 {
					t.Fatal("unverified brief created a deck")
				}
				return
			}
			replayed := generate(deckPrice.InternalCredits, tc.staleQuote)
			if replayed.Code != http.StatusOK || replayed.Body.String() != got.Body.String() || len(jobs.decks) != 1 {
				t.Fatalf("did not preserve one child on replay: %d %s / %d %s", got.Code, got.Body, replayed.Code, replayed.Body)
			}
			for _, deck := range jobs.decks {
				var booked webChatJobParams
				if !deck.IsWebPresentation() || json.Unmarshal(deck.Params, &booked) != nil || booked.ConversationID == chat.ID.String() ||
					!strings.Contains(booked.Prompt, fmt.Sprintf("Ровно %d слайдов", intake.Brief.SlideCount)) || (intake.Brief.Notes != "" && !strings.Contains(booked.Prompt, intake.Brief.Notes)) {
					t.Fatal("child lacks separate marked conversation and retained final brief")
				}
				deck.Status = domain.JobStatusAwaitingPayment
				resumed := generate(deckPrice.InternalCredits, tc.staleQuote)
				if resumed.Code != http.StatusOK || len(jobs.decks) != 1 || resumed.Body.String() != got.Body.String() || jobs.resumedID != deck.ID {
					t.Fatalf("payment retry started another deck or failed to resume it: %d %s", resumed.Code, resumed.Body)
				}
				if err := conversations.ArchiveConversationForAccount(ctx, owner, uuid.MustParse(booked.ConversationID), domain.ConversationSourceWeb); err != nil {
					t.Fatal(err)
				}
			}
			results.result.Artifacts[0].Deleted = true
			if after := read(); after.Code != http.StatusOK || !strings.Contains(after.Body.String(), `"presentation":`) || !strings.Contains(after.Body.String(), `"unavailable":true`) {
				t.Fatalf("durable child link was lost with the intake artifact: %d %s", after.Code, after.Body)
			}
		})
	}
}

type presentationChatTurnJobsStub struct {
	mutex     sync.Mutex
	job       *domain.Job
	decks     map[string]*domain.Job
	resumedID uuid.UUID
}

func (s *presentationChatTurnJobsStub) GetByIDForAccount(_ context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	if s.job.AccountID == accountID && s.job.ID == jobID {
		return s.job, nil
	}
	return nil, domain.ErrNotFound
}

func (s *presentationChatTurnJobsStub) GetLatestPresentationChatTurn(_ context.Context, accountID, conversationID uuid.UUID) (*domain.Job, error) {
	if ownedConversationJob(s.job, accountID, conversationID) {
		return s.job, nil
	}
	return nil, domain.ErrNotFound
}

func (s *presentationChatTurnJobsStub) GetByIdempotencyKeyForAccount(_ context.Context, accountID uuid.UUID, key string) (*domain.Job, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if deck := s.decks[key]; deck != nil && deck.AccountID == accountID {
		return deck, nil
	}
	return nil, domain.ErrNotFound
}

func (s *presentationChatTurnJobsStub) CreateJob(_ context.Context, input joborchestrator.CreateJobInput) (*domain.Job, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if deck := s.decks[input.IdempotencyKey]; deck != nil {
		return deck, nil
	}
	deck := newPersistedWebChatJob(input, uuid.New(), domain.JobStatusQueued)
	deck.PricingSnapshot, _ = json.Marshal(input.PricingSnapshot)
	s.decks[input.IdempotencyKey] = deck
	return deck, nil
}

func (s *presentationChatTurnJobsStub) PrepareAccountJob(context.Context, joborchestrator.PrepareAccountJobInput) (*domain.Job, error) {
	return nil, domain.ErrNotFound
}

func (s *presentationChatTurnJobsStub) RetryExpiredAccountImageJob(context.Context, uuid.UUID, uuid.UUID) (*domain.Job, error) {
	return nil, domain.ErrNotFound
}

func (s *presentationChatTurnJobsStub) ActivatePreparedAccountJob(_ context.Context, accountID, jobID uuid.UUID) (*domain.Job, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, job := range s.decks {
		if job.AccountID == accountID && job.ID == jobID && job.Status == domain.JobStatusAwaitingPayment {
			job.Status = domain.JobStatusQueued
			s.resumedID = job.ID
			return job, nil
		}
	}
	return nil, domain.ErrNotFound
}

func TestPresentationChatRechecksExactPersistedJobAfterConcurrentInsertConflict(t *testing.T) {
	h, _, _, stub, _ := newWebConversationMessageTestHandler(t)
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	id := providermodels.PublicTextGemini37Flash
	model, turn, err := textgeneration.Resolve(id, "topic", textgeneration.Models([]string{id}, prices), prices)
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	key := domain.WebPresentationIntakeJobKeyPrefix(owner) + uuid.NewString()
	params := newPresentationChatJobParams(uuid.New(), "topic", model)
	params.PresentationCredits = 5
	raw, _ := json.Marshal(params)
	winningJob := newPersistedWebChatJob(joborchestrator.CreateJobInput{
		AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode: domain.ResultModeAccountHistory, Operation: domain.OperationTextGenerate, Modality: domain.ModalityText,
		IdempotencyKey: key, CorrelationID: key, Params: raw,
	}, uuid.New(), domain.JobStatusQueued)
	winningJob.PricingSnapshot, _ = json.Marshal(turn)
	h.deps.ImageJobIdempotency = &presentationChatTurnJobsStub{decks: map[string]*domain.Job{key: winningJob}}
	stub.err = domain.ErrConflict
	if got, err := h.queuePresentationChatJob(context.Background(), owner, key, params, turn); err != nil || got.ID != winningJob.ID {
		t.Fatalf("exact replay missed the winning job: %#v, %v", got, err)
	}
	params.Prompt = "unrelated"
	if got, err := h.queuePresentationChatJob(context.Background(), owner, key, params, turn); !errors.Is(err, domain.ErrConflict) || got != nil {
		t.Fatalf("different prompt recovered unrelated job: %#v, %v", got, err)
	}
}
