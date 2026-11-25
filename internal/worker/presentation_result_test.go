package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/apimart"
	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/assistantfacts"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/textgeneration"
	"vk-ai-aggregator/internal/service/videorouter"
	"vk-ai-aggregator/internal/worker"
)

var errPresentationHistoryUnavailable = errors.New("temporary presentation history error")

type presentationHistoryRepo struct {
	domain.ConversationRepository
	failAnswer bool
}

func (r *presentationHistoryRepo) UpsertMessage(ctx context.Context, message *domain.ConversationMessage) (*domain.ConversationMessage, error) {
	if r.failAnswer && message.Role == domain.ConversationRoleAssistant {
		return nil, errPresentationHistoryUnavailable
	}
	return r.ConversationRepository.UpsertMessage(ctx, message)
}

type countedPresentationProvider struct {
	domain.Provider
	submits int
	polls   int
}

func (p *countedPresentationProvider) Submit(ctx context.Context, request domain.ProviderRequest) (domain.ProviderTask, error) {
	p.submits++
	return p.Provider.Submit(ctx, request)
}

func (p *countedPresentationProvider) Poll(ctx context.Context, ref domain.ProviderTaskRef) (domain.ProviderTaskResult, error) {
	p.polls++
	return p.Provider.Poll(ctx, ref)
}

type presentationProviderTransport func(*http.Request) (*http.Response, error)

func (f presentationProviderTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestPresentationCompletesFromExistingArtifactAfterHistoryWriteFailure(t *testing.T) {
	const topic = "учебный проект по 3д моделированию"
	prompt := "Подготовь презентацию на русском языке. Тема: " + topic + ".\n\nРовно 10 слайдов, включая обложку и вывод. Стиль: деловой.\n\n" +
		"Исходный материал и пожелания: " + strings.Repeat("Покажи этапы работы с кузовом и подбора материалов. ", 16) + "\n\n" +
		"Верни только слайды в Markdown. Каждый начинается со строки «## Заголовок». Ниже от одного до трёх коротких тезисов, каждый с новой строки через «- ». " +
		"На обложке достаточно одного подзаголовка. Не нумеруй заголовки. Не придумывай источники или статистику."
	const answer = `## Учебный проект по 3D-моделированию
- От эскиза к трёхмерной модели

## Задача проекта
- Задаём цель и ограничения

## Эскиз автомобиля
- Проверяем силуэт и пропорции

## Цифровая модель
- Строим основные поверхности

## Геометрия кузова
- Согласуем стыки и зазоры

## Материалы
- Сравниваем варианты отделки

## Проверка
- Смотрим модель с разных ракурсов

## Визуализация
- Подбираем освещение и фон

## Обсуждение
- Собираем замечания к модели

## Вывод
- Выбираем версию для дальнейшей работы`
	facts := presentationTestCatalogFacts(t)
	t.Run("ordinary chat", func(t *testing.T) {
		provider := &captureTextProvider{}
		h := newHarnessWithProvider(t, provider, func(d *worker.Deps) { d.AssistantFacts = facts })
		job := h.queueJob(t, domain.OperationTextGenerate, domain.ModalityText, "Какие модели есть в Epifera?")
		if err := h.gen.Process(context.Background(), taskFor(job)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(provider.last.TrustedFacts, "Факты НейроХаб") {
			t.Fatal("ordinary text generation lost its existing catalog context")
		}
		t.Logf("public catalog adds %d bytes to a %d-byte presentation brief", len(provider.last.TrustedFacts), len(prompt))
	})
	for _, scenario := range []string{"immediate Gemini", "provider polling"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			repo := &presentationHistoryRepo{ConversationRepository: memory.NewConversationRepo(), failAnswer: true}
			owner := uuid.New()
			conversation := &domain.Conversation{
				AccountID: owner, Source: domain.ConversationSourceWeb, Status: domain.ConversationActive,
			}
			if err := repo.CreateConversation(ctx, conversation); err != nil {
				t.Fatal(err)
			}
			params := map[string]string{
				"prompt":          prompt,
				"conversation_id": conversation.ID.String(), "conversation_source": "web",
			}
			provider := &countedPresentationProvider{Provider: mock.New(mock.WithCompleteAfterPolls(1))}
			job := &domain.Job{
				ID: uuid.New(), AccountID: owner, Source: "web", OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText,
				Status: domain.JobStatusQueued, ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
				IdempotencyKey: "web-presentation:" + owner.String() + ":" + uuid.NewString(),
			}
			var calls int
			if scenario == "immediate Gemini" {
				const publicModel = "gemini_2_5_flash_lite"
				const nativeModel = "gemini-2.5-flash-lite"
				transport := presentationProviderTransport(func(request *http.Request) (*http.Response, error) {
					calls++
					if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
						t.Errorf("unexpected Gemini request: %s %s", request.Method, request.URL.Path)
					}
					var requestBody struct {
						Model     string `json:"model"`
						Stream    *bool  `json:"stream"`
						MaxTokens int    `json:"max_tokens"`
						Messages  []struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						} `json:"messages"`
					}
					decoder := json.NewDecoder(request.Body)
					decoder.DisallowUnknownFields()
					if err := decoder.Decode(&requestBody); err != nil {
						t.Fatal(err)
					}
					if requestBody.Model != nativeModel || requestBody.Stream == nil || *requestBody.Stream ||
						requestBody.MaxTokens != domain.PaidTextMaxOutputTokens || len(requestBody.Messages) != 2 {
						t.Fatal("expected the documented non-streaming Gemini request and its paid output cap")
					}
					system, user := requestBody.Messages[0], requestBody.Messages[1]
					if system.Role != "system" || strings.Contains(system.Content, "Факты НейроХаб") || user.Role != "user" || user.Content != prompt {
						t.Fatal("the catalog was added to the presentation or the brief changed")
					}
					response, _ := json.Marshal(map[string]any{
						"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": answer}}},
						"usage":   map[string]int{"prompt_tokens": 80, "completion_tokens": 600},
					})
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(response)))}, nil
				})
				provider.Provider = apimart.New(apimart.Config{
					APIKey: "test-key", BaseURL: "https://provider.example/v1", EnabledTextModels: []string{nativeModel},
					HTTPClient: &http.Client{Transport: transport},
				})
				params["model_id"], params["model_code"], params["provider"] = publicModel, nativeModel, string(domain.ProviderAPIMart)
				snapshot, err := allTextTestPrices(t).Snapshot(textgeneration.Key(publicModel))
				if err != nil {
					t.Fatal(err)
				}
				snapshot, err = pricingcatalog.PresentationTextSnapshot(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				job.PricingSnapshot, _ = json.Marshal(snapshot)
				job.CostEstimate, job.CostReserved = snapshot.InternalCredits, snapshot.InternalCredits
			}
			job.Params, _ = json.Marshal(params)
			h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
				d.TextContext = dialogcontext.New(repo, dialogcontext.Config{Enabled: true})
				d.AssistantFacts = facts
			})
			if err := h.jobs.Create(ctx, job); err != nil {
				t.Fatal(err)
			}

			if err := h.gen.Process(ctx, taskFor(job)); !errors.Is(err, errPresentationHistoryUnavailable) {
				saved := h.reload(t, job.ID)
				t.Fatalf("expected deferred history write, got %v with status %s, error %q and %d HTTP calls", err, saved.Status, saved.ErrorCode, calls)
			}
			saved := h.reload(t, job.ID)
			if saved.Status != domain.JobStatusProviderSucceeded || len(saved.OutputArtifactIDs) != 1 || provider.submits != 1 {
				t.Fatalf("result was marked ready early or was not checkpointed: %s, %d artifacts, %d submits", saved.Status, len(saved.OutputArtifactIDs), provider.submits)
			}
			polls := provider.polls
			repo.failAnswer = false
			if err := h.poll.Process(ctx, taskFor(job)); err != nil {
				t.Fatal(err)
			}
			if got := h.reload(t, job.ID).Status; got != domain.JobStatusResultReady {
				t.Fatalf("status after retry = %s", got)
			}
			messages, err := repo.ListMessagesAfter(ctx, conversation.ID, 0, 10)
			if err != nil || len(messages) != 2 || messages[1].Role != domain.ConversationRoleAssistant || messages[1].JobID != job.ID || messages[1].Text == "" {
				t.Fatalf("assistant answer was not stored for the existing job: %v", err)
			}
			if scenario == "immediate Gemini" && (messages[1].Text != answer || calls != 1) {
				t.Fatalf("expected one Gemini call and its moderated answer, got %d HTTP calls", calls)
			}
			if provider.submits != 1 || provider.polls != polls {
				t.Fatalf("history recovery repeated provider calls: %d submits, %d polls (initial %d)", provider.submits, provider.polls, polls)
			}
		})
	}
}

func presentationTestCatalogFacts(t *testing.T) *assistantfacts.Builder {
	t.Helper()
	prices := staticWorkerPricingCatalog(t)
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}

	images := make(map[string]bool)
	imageProviders := make(map[domain.ProviderName]bool)
	for _, model := range modelcatalog.ListPublicModels(domain.OperationImageGenerate) {
		images[model.ModelID] = true
		imageProviders[model.Provider] = true
	}
	videos := make(map[domain.VideoRouteAlias]bool)
	videoProviders := make(map[domain.ProviderName]videorouter.ProviderConfig)
	for _, route := range videorouter.DefaultRouteSpecs() {
		videos[route.Alias] = true
		videoProviders[route.Provider] = videorouter.ProviderConfig{Enabled: true}
	}
	videoCatalog, err := videorouter.NewCatalog(videorouter.Config{
		Pricing: prices, RouterEnabled: true, Providers: videoProviders, EnabledRoutes: videos,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog := productcatalog.New(productcatalog.Config{
		PricingCatalog: prices, VideoRoutes: videoCatalog.PublicRoutes(),
		EnabledImageModels: images, ImageProviderReady: imageProviders,
	})
	t.Logf("fixture has %d image models and %d video routes", len(catalog.ImageModels()), len(catalog.VideoRoutes()))
	return assistantfacts.New(assistantfacts.Config{Catalog: catalog, PricingCatalog: prices})
}
