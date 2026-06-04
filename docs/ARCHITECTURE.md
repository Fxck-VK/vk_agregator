Ниже — **максимальная production-архитектура** для агрегатора нейросетей во ВКонтакте. Я бы проектировал это не как “бота”, а как **AI Job Processing Platform**: платформа принимает сообщения из VK, превращает их в задачи, выбирает провайдера/модель, контролирует лимиты и деньги, получает артефакты — текст, фото, видео, аудио — и доставляет результат обратно пользователю.

Сразу позиция: **основной backend — Go**. Python можно добавить отдельными worker’ами для локальной обработки медиа, ffmpeg-обвязки, ML-инструментов, но orchestration/backend/provider gateway/job processing лучше держать на Go.

> **Статус реализации:** этот документ описывает целевую production-архитектуру
> и архитектурные инварианты. Актуальное состояние кода, выполненные шаги,
> ограничения и ближайшие задачи см. в `PROGRESS.md`, `AUDIT.md`,
> `ROADMAP.md` и `TASKS.md`.

---

# 1. Главная идея архитектуры

Не должно быть такого:

```text
VK handler -> вызвать Kling/OpenAI/Gemini -> дождаться -> отправить пользователю
```

Это сломается на первом же видео/таймауте/429/рестарте сервера.

Правильная модель:

```text
VK Event
  -> Inbound Event
  -> Command
  -> Job
  -> Workflow
  -> Provider Task
  -> Artifact
  -> Delivery
  -> User Notification
```

То есть **любое действие пользователя превращается в Job**.

Текст — это короткий job.
Генерация фото — средний job.
Генерация видео через Kling/Veo/Runway/Pika/etc — долгий job.
Multi-step сценарий “сначала картинка, потом оживить картинку в видео, потом upscale” — это workflow из нескольких job/activity.

---

# 2. Почему архитектура должна быть именно такой

У тебя будут разные классы нейросетей:

```text
Text:
  - OpenAI
  - Gemini
  - Claude
  - Mistral
  - локальные LLM, если потом появятся

Image:
  - OpenAI image models
  - Gemini / Nano Banana
  - Midjourney-like API через посредников
  - Stable Diffusion / ComfyUI, если свой GPU

Video:
  - Kling
  - Veo
  - Runway
  - Pika
  - Luma
  - другие text-to-video / image-to-video сервисы

Audio:
  - TTS
  - STT
  - music generation
  - lip sync / voiceover

Post-processing:
  - upscale
  - watermark
  - resize/crop
  - transcode
  - compression
```

У провайдеров разные форматы API: OpenAI Responses API умеет работать с text/image inputs и создавать text/JSON outputs; OpenAI image generation доступна через Images API или Responses API; Google описывает Nano Banana как native image generation в Gemini API, где можно работать с текстом, изображениями, видео или их комбинацией; Kling в developer API имеет разделы для video workflows, включая Text-to-Video и Image-to-Video. Значит, тебе нужен **единый provider abstraction layer**, а не прямые вызовы из бизнес-логики. ([OpenAI Platform][1])

---

# 3. Максимальная схема системы

```text
                         ┌──────────────────────┐
                         │        VK User        │
                         └───────────┬──────────┘
                                     │
                                     ▼
┌──────────────────────────────────────────────────────────────┐
│                         EDGE LAYER                            │
│  CDN/WAF, TLS, rate limit by IP, bot protection, ingress logs  │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                    VK INBOUND GATEWAY                         │
│  Callback API / Long Poll / VK event verification / idempotency│
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                    COMMAND ROUTER                             │
│  parses message, attachments, buttons, commands, user state    │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                    JOB ORCHESTRATOR                           │
│  creates jobs, validates quota, reserves credits, starts flow   │
└───────────────┬────────────────────────────┬─────────────────┘
                │                            │
                ▼                            ▼
      ┌───────────────────┐        ┌────────────────────┐
      │  WORKFLOW ENGINE   │        │   EVENT BUS / LOG   │
      │ Temporal / custom  │        │ Kafka / Redpanda    │
      └─────────┬─────────┘        └──────────┬─────────┘
                │                             │
                ▼                             ▼
┌──────────────────────────────────────────────────────────────┐
│                    WORKER POOLS                               │
│  text workers / image workers / video workers / media workers  │
└───────────────┬──────────────────────┬───────────────────────┘
                │                      │
                ▼                      ▼
┌──────────────────────────┐   ┌───────────────────────────────┐
│     PROVIDER GATEWAY      │   │       MEDIA PIPELINE           │
│  OpenAI/Gemini/Kling/etc  │   │ download, scan, resize, ffmpeg │
└───────────────┬──────────┘   └───────────────┬───────────────┘
                │                              │
                ▼                              ▼
┌──────────────────────────────────────────────────────────────┐
│                    STORAGE LAYER                              │
│ Postgres, Redis, S3/MinIO, ClickHouse, Vector DB, audit logs   │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                    DELIVERY SERVICE                           │
│  upload photo/video/doc to VK, messages.send, retry, status    │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
                         ┌───────────┐
                         │  VK User  │
                         └───────────┘
```

---

# 4. Главные сервисы

## 4.1. Edge Gateway

Отвечает за внешний вход:

```text
- TLS
- reverse proxy
- request size limit
- базовый rate limit
- WAF
- IP allow/block rules
- health checks
- access logs
```

Стек:

```text
Cloudflare / Nginx / Traefik / Envoy
```

Для VK callback endpoint нужно быстро принимать запрос и не делать тяжёлую работу в HTTP handler. Это общий принцип webhook-архитектуры: например, OpenAI в своей документации по webhooks прямо рекомендует быстро отвечать 2xx и выносить нетривиальную обработку в background worker; также webhook-события могут ретраиться и иногда приходить дублями, поэтому нужна idempotency. ([OpenAI Platform][2])

---

## 4.2. VK Inbound Gateway

Это отдельный сервис/модуль, который знает только VK.

Задачи:

```text
- принять Callback API event;
- проверить group_id / secret / confirmation;
- сохранить raw event в inbound_events;
- создать idempotency key;
- быстро ответить VK;
- передать событие дальше в Command Router;
- не вызывать нейросети;
- не списывать деньги;
- не отправлять долгие ответы.
```

VK Callback API при первичной настройке требует подтверждение сервера, а для остальных событий обычно ожидается быстрый ответ `ok`; это поведение также описано в документации/SDK VK Callback API. ([packagist.org][3])

Пример ответственности:

```text
POST /webhooks/vk/{group_id}

1. Read raw body.
2. Validate source.
3. Parse event.
4. Save inbound event.
5. Enqueue event.
6. Return confirmation/ok.
```

---

## 4.3. Command Router

Он превращает сообщение пользователя в команду.

Примеры команд:

```text
/text.ask
/image.generate
/image.edit
/video.generate
/video.image_to_video
/video.extend
/audio.tts
/balance
/cancel
/status
/help
```

Он решает:

```text
- что хотел пользователь;
- есть ли вложения;
- какой режим выбран;
- какой provider/model нужен;
- нужно ли уточнение;
- можно ли запускать задачу;
- нужно ли показать меню/кнопки.
```

Важно: Command Router **не вызывает AI API напрямую**. Он создаёт нормализованный `Command`.

---

## 4.4. User / Identity Service

Хранит пользователей VK и их состояние.

```text
users:
  id
  vk_user_id
  first_seen_at
  last_seen_at
  status
  locale
  timezone
  risk_level
  created_at

vk_conversations:
  id
  vk_peer_id
  user_id
  last_message_id
  current_mode
  current_scene
  created_at
```

Зачем:

```text
- баланс;
- лимиты;
- история;
- подписка;
- бан;
- антиспам;
- настройки качества;
- выбранная нейросеть по умолчанию.
```

---

## 4.5. Conversation Service

Отвечает за контекст диалога.

Для текстовых моделей это важно:

```text
- хранить историю;
- резать историю по токенам;
- делать summary;
- хранить system prompt;
- хранить mode/persona;
- отделять личные диалоги от групповых чатов.
```

Хранилища:

```text
Postgres:
  canonical history

Redis:
  hot context cache

Vector DB / pgvector:
  long-term memory / retrieval, если нужно
```

---

## 4.6. Prompt Service

Не надо хранить промпты прямо в коде.

Нужен отдельный слой:

```text
prompt_templates:
  id
  code
  version
  modality
  system_prompt
  user_prompt_template
  provider_specific_params
  safety_rules
  created_at

prompt_runs:
  id
  job_id
  template_id
  template_version
  final_prompt
  variables_json
  created_at
```

Зачем:

```text
- A/B тесты промптов;
- версионирование;
- откат;
- разные промпты для OpenAI/Gemini/Kling;
- аудит, почему результат был таким;
- настройка без переписывания кода.
```

---

## 4.7. Model Catalog Service

Это очень важный компонент для агрегатора.

Он хранит не “OpenAI/Gemini/Kling” как строки в коде, а полноценный каталог моделей и возможностей.

```text
providers:
  id
  code              -- openai, google, kling, runway
  display_name
  status            -- active, degraded, disabled
  base_url
  auth_type
  created_at

models:
  id
  provider_id
  code              -- gpt-5.5, gemini-3.1-flash-image, kling-v2...
  display_name
  modality          -- text, image, video, audio
  operation         -- generate, edit, image_to_video, upscale
  status
  quality_tier
  latency_tier
  created_at

model_capabilities:
  model_id
  supports_text_input
  supports_image_input
  supports_video_input
  supports_audio_input
  supports_text_output
  supports_image_output
  supports_video_output
  max_prompt_chars
  max_images
  supported_aspect_ratios
  max_duration_sec
  supports_negative_prompt
  supports_seed
  supports_webhook
  supports_polling
```

Почему это нужно: Google, например, описывает несколько Nano Banana моделей с разным назначением: `gemini-3.1-flash-image` как high-efficiency модель, `gemini-3-pro-image` как professional asset production, и `gemini-2.5-flash-image` как speed/efficiency вариант; такую разницу нельзя нормально поддерживать строками в коде. ([Google AI for Developers][4])

---

## 4.8. Job Orchestrator

Это центр системы.

Он делает:

```text
- создаёт job;
- валидирует пользователя;
- проверяет баланс;
- резервирует кредиты;
- выбирает workflow;
- ставит задачу в очередь;
- меняет статусы;
- публикует события;
- обрабатывает cancel/status/retry;
- гарантирует idempotency.
```

Пример job:

```text
jobs:
  id
  user_id
  vk_peer_id
  command_id
  operation_type       -- text_generate, image_generate, image_edit, video_generate
  modality             -- text, image, video, audio
  provider_id
  model_id
  status
  priority
  idempotency_key
  correlation_id
  input_artifact_ids
  output_artifact_ids
  cost_estimate
  cost_reserved
  cost_captured
  error_code
  error_message
  created_at
  updated_at
  expires_at
```

Статусы:

```text
received
validated
rejected
awaiting_payment
credits_reserved
queued
dispatching_provider
provider_submitted
provider_pending
provider_processing
provider_succeeded
provider_failed
postprocessing
result_ready
delivering
succeeded
failed_retryable
failed_terminal
cancelled
expired
refunded
```

---

## 4.9. Workflow Engine

Для максимальной архитектуры я бы использовал **Temporal** или аналогичный workflow engine.

Почему: видео-задачи могут жить минуты, десятки минут, иногда дольше. Там есть много шагов:

```text
1. скачать входное изображение;
2. проверить безопасность;
3. подготовить prompt;
4. зарезервировать деньги;
5. отправить задачу в Kling;
6. сохранить provider_task_id;
7. ждать webhook или polling;
8. скачать видео;
9. проверить файл;
10. перекодировать;
11. загрузить в VK;
12. отправить сообщение;
13. списать деньги;
14. записать аналитику.
```

Если сервер упал на шаге 7 или 10, процесс должен продолжиться, а не потеряться. Temporal описывает workflow execution как durable/reliable/scalable function execution, а retry policy — как настройки повторов после ошибок в workflow/activity. ([Temporal Docs][5])

Для Codex это тоже удобно: можно давать ему конкретные workflow:

```text
Implement ImageGenerationWorkflow
Implement VideoGenerationWorkflow
Implement ProviderPollingWorkflow
Implement DeliveryWorkflow
Implement BillingReservationWorkflow
```

---

## 4.10. Provider Gateway

Это слой, который изолирует весь проект от хаоса внешних AI API.

```text
Provider Gateway
  ├── openai_adapter
  ├── google_gemini_adapter
  ├── kling_adapter
  ├── runway_adapter
  ├── pika_adapter
  ├── local_comfyui_adapter
  └── mock_provider_adapter
```

Бизнес-логика не должна знать, как именно Kling принимает `image_url`, как Gemini возвращает inline image bytes, как OpenAI отдаёт output item, или как конкретный провайдер называет статус.

Внутри проекта должен быть единый контракт.

Пример Go-интерфейса:

```go
type Provider interface {
    Name() ProviderName
    Capabilities(ctx context.Context) ([]Capability, error)

    Estimate(ctx context.Context, req ProviderRequest) (CostEstimate, error)

    Submit(ctx context.Context, req ProviderRequest) (ProviderTask, error)
    Poll(ctx context.Context, task ProviderTaskRef) (ProviderTaskStatus, error)
    Cancel(ctx context.Context, task ProviderTaskRef) error
}
```

Для webhook-провайдеров:

```go
type WebhookProvider interface {
    Provider
    VerifyWebhook(ctx context.Context, headers http.Header, body []byte) error
    ParseWebhook(ctx context.Context, body []byte) (ProviderWebhookEvent, error)
}
```

---

# 5. Provider Gateway должен уметь не просто “вызвать API”

Его реальные обязанности:

```text
- нормализация request/response;
- provider-specific auth;
- provider-specific retry;
- provider-specific rate limits;
- circuit breaker;
- timeout policy;
- fallback policy;
- cost estimation;
- idempotency;
- polling;
- webhook verification;
- download output artifacts;
- provider health tracking;
- mapping provider error -> internal error.
```

OpenAI rate limits, например, могут считаться по RPM, RPD, TPM, TPD, IPM и другим метрикам, а также различаться по модели и проекту; поэтому лимиты нельзя размазать по worker’ам, их надо централизовать в Provider Gateway/Rate Limit Service. ([OpenAI Platform][6])

---

# 6. Provider Router

Отдельный компонент, который выбирает, куда отправить задачу.

Вход:

```text
operation_type: video_generate
input: text + image
user_tier: pro
quality: high
latency_preference: normal
budget_limit: 100 credits
aspect_ratio: 9:16
duration: 10 sec
provider_preference: auto
```

Выход:

```text
provider: kling
model: kling-video-pro
fallbacks:
  1. runway
  2. pika
  3. google_veo
```

Факторы выбора:

```text
- поддерживает ли модель нужную операцию;
- цена;
- ожидаемая latency;
- текущие rate limits;
- provider health;
- качество;
- user tier;
- региональные ограничения;
- доступность webhook/polling;
- лимит длительности видео;
- лимит aspect ratio;
- лимит входных изображений.
```

Важно: fallback должен быть **явным**. Если пользователь выбрал именно Kling, ты не должен молча отправлять в другую модель. Если пользователь выбрал “авто”, можно делать fallback.

---

# 7. Media Pipeline

Это отдельная подсистема, потому что фото/видео быстро становятся самой тяжёлой частью.

```text
Media Pipeline
  ├── media_ingest_worker
  ├── media_scan_worker
  ├── image_preprocess_worker
  ├── video_preprocess_worker
  ├── media_transcode_worker
  ├── thumbnail_worker
  ├── artifact_packager_worker
  └── vk_upload_worker
```

## Что делает Media Pipeline

```text
- скачать вложение из VK;
- проверить MIME type;
- проверить размер;
- вычислить sha256;
- сохранить оригинал в S3/MinIO;
- сделать preview/thumbnail;
- удалить EXIF, если нужно;
- привести картинку к требованиям provider API;
- скачать результат от provider;
- перекодировать видео через ffmpeg;
- сделать fallback-формат;
- подготовить файл для VK;
- сохранить artifact metadata.
```

Таблицы:

```text
artifacts:
  id
  owner_user_id
  job_id
  kind                 -- input, output, intermediate
  media_type           -- image, video, audio, text, document
  mime_type
  storage_bucket
  storage_key
  public_url
  sha256
  size_bytes
  width
  height
  duration_ms
  status
  created_at

artifact_variants:
  id
  artifact_id
  variant_type         -- original, preview, vk_photo, vk_doc, vk_video, thumbnail
  storage_key
  mime_type
  size_bytes
  width
  height
  duration_ms
```

---

# 8. VK Delivery Service

Это отдельный сервис, который знает, как правильно доставить результат во ВКонтакте.

VK `messages.send` имеет `random_id`, который используется как уникальный идентификатор, чтобы избежать повторной отправки сообщения; также VK attachment передаётся отдельным параметром. Для фото в личное сообщение VK использует цепочку `photos.getMessagesUploadServer` → upload → `photos.saveMessagesPhoto`; для документов есть `docs.getMessagesUploadServer`; для видео SDK VK указывает, что `video.save` возвращает адрес сервера для загрузки. ([GitHub][7])

Delivery Service делает:

```text
- отправить "принял задачу";
- отправить "генерация началась";
- отправить "ещё работаю";
- загрузить фото как VK photo attachment;
- загрузить видео как VK video или document;
- отправить файл;
- обработать ошибки VK;
- не отправить дубль;
- сохранить delivery status.
```

Таблицы:

```text
deliveries:
  id
  job_id
  user_id
  vk_peer_id
  type              -- message, photo, video, doc
  status            -- pending, uploading, sent, failed, retrying
  vk_random_id
  vk_message_id
  attachment
  error_code
  created_at
  updated_at

vk_uploads:
  id
  artifact_id
  delivery_id
  upload_type       -- photo_message, doc_message, video
  status
  upload_server_url_hash
  vk_owner_id
  vk_media_id
  vk_access_key
  attachment_string
```

---

# 9. Очереди и event bus

Для максимальной архитектуры я бы разделил **task queues** и **event log**.

## Task queues

Для задач, которые надо выполнить:

```text
queue.text.generate
queue.image.generate
queue.image.edit
queue.video.generate
queue.video.poll
queue.media.preprocess
queue.media.transcode
queue.vk.delivery
queue.billing.capture
queue.billing.refund
queue.moderation.input
queue.moderation.output
```

## Event bus

Для событий системы:

```text
event.inbound.received
event.command.created
event.job.created
event.job.status_changed
event.provider.task_submitted
event.provider.task_completed
event.artifact.created
event.delivery.sent
event.billing.reserved
event.billing.captured
event.billing.refunded
event.moderation.rejected
event.provider.rate_limited
event.job.failed
```

Практичный production-вариант:

```text
Temporal:
  long-running workflows

Kafka / Redpanda:
  event log, analytics, audit, async projections

Redis:
  cache, rate limit, short locks

RabbitMQ / NATS JetStream:
  можно использовать как task queue, если не хочешь все задачи тащить через Kafka
```

Kafka официально поддерживает репликацию данных по partition/topic, а типичный production replication factor часто равен 3; Redis Streams поддерживает consumer groups; RabbitMQ имеет Dead Letter Exchange для сообщений, которые нельзя обработать. Это всё применимо к твоей системе, потому что AI-задачи будут падать, ретраиться и иногда уходить в DLQ. ([Apache Kafka][8])

---

# 10. Почему нельзя делать одну очередь на всё

Плохая схема:

```text
queue.jobs
```

Проблема:

```text
- video job может висеть 10 минут;
- text job должен отвечать быстро;
- image job средний;
- delivery job зависит от VK;
- polling job зависит от provider;
- transcode job грузит CPU;
- billing job должен быть строгим и быстрым.
```

Правильнее:

```text
text workers:
  много, быстрые, низкая latency

image workers:
  среднее количество, controlled concurrency

video workers:
  мало, дорогие, долгие, строгие лимиты

poller workers:
  отдельный пул, не блокирует generation workers

media workers:
  CPU-heavy, autoscale по queue depth

delivery workers:
  отдельные лимиты под VK API
```

---

# 11. Billing / Credits / Ledger

Для такого сервиса биллинг нельзя делать полем `balance` и `balance -= 10`.

Нужен ledger.

```text
credit_accounts:
  id
  user_id
  currency          -- credits, rub, stars, internal
  balance_cached
  created_at

ledger_entries:
  id
  account_id
  job_id
  type              -- topup, reserve, capture, release, refund, adjustment
  amount
  status            -- pending, committed, cancelled
  idempotency_key
  reason
  created_at

credit_reservations:
  id
  account_id
  job_id
  amount
  status            -- reserved, captured, released, expired
  expires_at
  created_at
```

Правильный flow:

```text
1. Estimate cost.
2. Reserve credits.
3. Start job.
4. If success -> capture.
5. If provider failed before useful output -> release/refund.
6. If partial result -> capture partial or refund by policy.
```

Пример:

```text
User asks video generation.
Estimated cost: 80 credits.
System reserves 80.
Kling returns success.
System captures 80.
Delivery fails temporarily.
No refund yet, because artifact exists.
VK delivery retries.
```

---

# 12. Pricing Service

Цены не должны быть захардкожены.

```text
pricing_rules:
  id
  provider_id
  model_id
  operation_type
  user_tier
  base_price_credits
  per_second_credits
  per_image_credits
  per_token_credits
  markup_percent
  active_from
  active_to
```

Нужно учитывать:

```text
- себестоимость provider API;
- retry cost;
- failed task policy;
- cashback/refund;
- разные тарифы пользователей;
- premium queue;
- скидки;
- бесплатные лимиты;
- лимит дневных генераций.
```

---

# 13. Rate Limit Service

Должны быть разные лимиты.

```text
User limits:
  - запросов в минуту;
  - генераций фото в день;
  - генераций видео в день;
  - одновременных активных job;
  - максимальная длительность видео.

Provider limits:
  - RPM;
  - TPM;
  - IPM;
  - concurrent jobs;
  - daily spend cap;
  - per-model cap.

VK limits:
  - messages send rate;
  - upload rate;
  - anti-spam delay.

System limits:
  - queue depth;
  - CPU transcode slots;
  - storage bandwidth;
  - total daily spend.
```

Таблицы:

```text
rate_limit_buckets:
  key
  limit_type
  capacity
  refill_rate
  current_tokens
  updated_at

provider_limit_state:
  provider_id
  model_id
  rpm_remaining
  tpm_remaining
  ipm_remaining
  reset_at
  observed_at
```

---

# 14. Moderation / Safety Service

Для VK-агрегатора это обязательно, особенно если будут фото/видео.

Нужно проверять:

```text
Input:
  - текст пользователя;
  - прикреплённые фото;
  - прикреплённые видео;
  - ссылки;
  - prompt injection;
  - запрещённый контент;
  - попытки deepfake/NSFW/несовершеннолетние/обман.

Output:
  - сгенерированное изображение;
  - сгенерированное видео;
  - текстовый ответ;
  - metadata provider-а;
  - watermark/labeling, если требуется.
```

Результаты:

```text
moderation_results:
  id
  job_id
  artifact_id
  stage             -- input, output
  decision          -- allow, block, review, sanitize
  categories_json
  provider
  raw_response_json
  created_at
```

Инвариант: **ничего не отправлять пользователю, пока output moderation не прошёл.**

---

# 15. Admin Panel

Без админки ты не сможешь нормально управлять системой.

Нужны разделы:

```text
Users:
  - баланс;
  - история job;
  - бан/разбан;
  - риск;
  - лимиты.

Jobs:
  - статус;
  - provider task id;
  - attempts;
  - input/output artifacts;
  - retry/cancel/refund.

Providers:
  - health;
  - latency;
  - error rate;
  - spend;
  - лимиты;
  - включить/выключить модель.

Billing:
  - ledger;
  - refunds;
  - manual adjustments.

Prompts:
  - версии;
  - A/B tests;
  - откат.

Moderation:
  - rejected jobs;
  - manual review;
  - audit trail.

System:
  - queues;
  - DLQ;
  - feature flags;
  - incident mode.
```

---

# 16. Observability

Production без observability — это слепой проект.

Нужны:

```text
Logs:
  - structured JSON logs;
  - correlation_id;
  - job_id;
  - user_id hash, не raw PII;
  - provider;
  - model;
  - status;
  - error_code.

Metrics:
  - requests per second;
  - job latency by modality;
  - provider latency;
  - provider error rate;
  - queue depth;
  - active jobs;
  - spend per provider;
  - delivery failures;
  - moderation reject rate;
  - billing mismatch count.

Tracing:
  - VK inbound -> job -> provider -> artifact -> delivery;
  - trace_id в каждом сервисе;
  - spans на внешние API вызовы.

Alerts:
  - provider error rate > threshold;
  - queue depth growing;
  - VK delivery failures;
  - billing capture failed;
  - artifact download failed;
  - DLQ not empty;
  - daily spend cap near limit.
```

OpenTelemetry — vendor-neutral observability framework для traces/metrics/logs, Prometheus хранит метрики как time series с labels, а Kubernetes HPA может масштабировать workload по CPU/memory/custom metrics. Это хорошо ложится на твою задачу: worker’ы можно масштабировать по queue depth и provider backpressure, а не только по CPU. ([OpenTelemetry][9])

---

# 17. Storage Layer

## Postgres

Главная транзакционная база:

```text
- users
- conversations
- commands
- jobs
- job_attempts
- provider_tasks
- artifacts metadata
- billing ledger
- deliveries
- moderation results
- outbox events
- idempotency keys
```

## Redis

```text
- cache;
- rate limit buckets;
- short locks;
- hot job status;
- session state;
- temporary provider limit state.
```

## S3 / MinIO

```text
- original user images;
- generated images;
- generated videos;
- thumbnails;
- intermediate files;
- provider raw outputs;
- exports.
```

## Kafka / Redpanda

```text
- immutable event log;
- analytics events;
- projections;
- audit stream;
- async integrations.
```

## ClickHouse

```text
- analytics;
- cost reports;
- provider performance;
- user behavior;
- funnel analysis;
- latency reports.
```

## OpenSearch / Loki

```text
- logs search;
- incident debugging;
- error aggregation.
```

## Vector DB / pgvector

```text
- long-term memory;
- semantic search over user history;
- prompt examples;
- generated asset search.
```

---

# 18. Idempotency

Это один из главных production-инвариантов.

Ты должен ожидать, что:

```text
- VK пришлёт событие повторно;
- пользователь нажмёт кнопку два раза;
- provider webhook придёт два раза;
- worker упадёт после вызова provider API, но до записи в БД;
- delivery отправит сообщение, но не успеет сохранить статус;
- retry создаст повторную попытку.
```

Поэтому нужны idempotency keys.

```text
idempotency_keys:
  key
  scope             -- inbound_event, job_create, provider_submit, vk_delivery
  resource_type
  resource_id
  status
  created_at
  expires_at
```

Примеры ключей:

```text
vk_event:{group_id}:{event_id}
job:{user_id}:{message_id}:{command_hash}
provider_submit:{job_id}:{attempt_no}
vk_delivery:{job_id}:{delivery_type}
billing_capture:{job_id}:{reservation_id}
```

Инвариант: **любая внешняя операция должна быть безопасна при повторе.**

---

# 19. Outbox / Inbox pattern

Для серьёзного backend это обязательно.

Проблема:

```text
1. Ты записал job в Postgres.
2. Потом хотел отправить event в Kafka.
3. Между этими действиями сервис упал.
4. Job есть, event потерян.
```

Решение:

```text
В одной транзакции:
  - записать job;
  - записать outbox_event.

Потом отдельный publisher:
  - читает outbox_events;
  - публикует в Kafka/queue;
  - помечает published.
```

Таблица:

```text
outbox_events:
  id
  aggregate_type
  aggregate_id
  event_type
  payload_json
  status
  attempts
  next_attempt_at
  created_at
  published_at
```

Для входящих webhook/provider events — `inbox_events`, чтобы дедуплицировать вход.

---

# 20. Видео workflow на примере Kling

Kling-like video flow:

```text
User sends:
  "Сделай видео: девушка идёт по ночному Токио, 9:16, 10 секунд"

System:
  1. VK Inbound Gateway сохраняет сообщение.
  2. Command Router определяет video_generate.
  3. Job Orchestrator создаёт job.
  4. Billing резервирует credits.
  5. Provider Router выбирает Kling.
  6. Workflow запускает VideoGenerationWorkflow.
  7. Provider Gateway отправляет task в Kling.
  8. Сохраняется provider_task_id.
  9. Poller/Webhook Receiver ждёт завершения.
  10. Media Pipeline скачивает video artifact.
  11. Output moderation проверяет результат.
  12. Transcode worker готовит формат для VK.
  13. VK Delivery загружает видео/документ.
  14. Billing capture.
  15. User получает результат.
```

Kling developer API документирует отдельные video generation направления, включая Text-to-Video и Image-to-Video, поэтому для таких провайдеров нужно проектировать именно асинхронный provider task lifecycle, а не обычный request/response. ([kling.ai][10])

---

# 21. Text workflow

Текст можно делать быстрее, но я всё равно рекомендую приводить его к job-модели.

```text
User:
  "Напиши пост для VK"

Flow:
  1. inbound event
  2. command: text_generate
  3. job created
  4. credits reserved
  5. text worker calls provider
  6. result saved
  7. delivery sends text
  8. billing capture
```

Можно добавить fast path:

```text
Если задача дешёвая и быстрая:
  - handler создаёт job;
  - worker берёт почти мгновенно;
  - пользователь видит ответ через 1-5 секунд.
```

Но даже fast path не должен обходить:

```text
- idempotency;
- billing;
- logs;
- moderation;
- job status.
```

---

# 22. Image workflow

```text
User sends:
  "Сделай фото в стиле luxury ad"

Flow:
  1. Save inbound event.
  2. Parse command.
  3. Create image_generate job.
  4. Reserve credits.
  5. Build prompt from template.
  6. Choose provider/model.
  7. Submit to provider.
  8. Receive image bytes/url.
  9. Save artifact to S3.
  10. Moderate output.
  11. Prepare VK photo variant.
  12. Upload via VK photo message upload.
  13. Send message with attachment.
  14. Capture credits.
```

Для Gemini/Nano Banana важно, что ответ может содержать как текстовые части, так и inline image data; в Google docs для Go-примера результат проходит по `Content.Parts`, где часть может быть `Text` или `InlineData`. Это ещё один аргумент за нормализованный artifact layer. ([Google AI for Developers][4])

---

# 23. Multi-stage workflow

Пример сложного продукта:

```text
"Сделай аватар, потом оживи его в видео, потом добавь музыку"
```

Workflow:

```text
ImageGenerationWorkflow
  -> ImageModerationActivity
  -> ImageUpscaleActivity
  -> ImageToVideoWorkflow
  -> VideoModerationActivity
  -> AudioGenerationActivity
  -> MuxAudioVideoActivity
  -> DeliveryWorkflow
```

В БД это не один job, а дерево:

```text
workflow_runs:
  id
  root_job_id
  status
  current_step
  created_at

jobs:
  image job
  upscale job
  video job
  audio job
  mux job
  delivery job
```

---

# 24. Provider errors

Нужно сразу сделать нормальную классификацию ошибок.

```text
provider_error_classes:
  - rate_limited
  - auth_failed
  - insufficient_provider_balance
  - invalid_request
  - content_rejected
  - provider_overloaded
  - provider_timeout
  - provider_internal_error
  - task_not_found
  - output_download_failed
  - unsupported_capability
```

Retry policy:

```text
rate_limited:
  retry with backoff, respect reset time

provider_overloaded:
  retry or fallback

invalid_request:
  no retry, mark failed_terminal

content_rejected:
  no retry, maybe refund by policy

provider_timeout:
  poll again or retry submit only if idempotency safe

auth_failed:
  disable provider key, alert

insufficient_provider_balance:
  disable provider account, alert finance
```

---

# 25. Circuit Breaker

Provider Gateway должен временно отключать провайдера, если он деградирует.

```text
provider_health:
  provider_id
  model_id
  status              -- healthy, degraded, open_circuit, disabled
  error_rate_5m
  p95_latency_5m
  rate_limit_state
  last_success_at
  last_failure_at
```

Правило:

```text
Если Kling даёт 50% ошибок за 5 минут:
  - перевести модель в degraded;
  - новые auto jobs отправлять в fallback;
  - exact-provider jobs ставить в очередь или показывать пользователю предупреждение;
  - alert в Telegram/Slack.
```

---

# 26. Security

Обязательные компоненты:

```text
- secrets manager;
- API keys только в env/secret storage;
- запрет логирования provider keys;
- raw webhook body сохранять аккуратно;
- signature verification для provider webhooks;
- idempotency для webhook-id;
- RBAC для админки;
- audit log для ручных действий;
- PII minimization;
- encryption at rest для чувствительных данных;
- signed URLs для S3;
- malware scan для входящих файлов;
- лимит размера upload;
- content-type sniffing, не верить MIME из запроса;
- SSRF защита при скачивании URL от provider;
- egress allowlist для provider domains.
```

OpenAI webhooks используют signing secret и заголовки для проверки происхождения запроса; в документации также указано, что webhook endpoint должен дедуплицировать возможные дубликаты по `webhook-id`. Это хороший шаблон и для любых других provider webhooks. ([OpenAI Platform][2])

---

# 27. Deployment architecture

Максимальный production вариант:

```text
Kubernetes cluster

Namespaces:
  prod
  staging
  monitoring
  data
  security

Deployments:
  api-gateway
  vk-inbound-gateway
  command-router
  job-orchestrator
  provider-gateway
  provider-webhook-receiver
  text-worker
  image-worker
  video-worker
  poller-worker
  media-worker
  delivery-worker
  billing-worker
  admin-api
  admin-frontend

Stateful:
  Postgres cluster
  Redis cluster
  Kafka/Redpanda
  Temporal
  MinIO/S3 external
  ClickHouse
  Prometheus
  Grafana
  Loki/OpenSearch
```

Scaling:

```text
api-gateway:
  scale by RPS

vk-inbound-gateway:
  scale by inbound events/sec

text-worker:
  scale by queue.text.generate depth

image-worker:
  scale by queue.image.generate depth and provider limits

video-worker:
  scale by active video jobs and provider concurrency

media-worker:
  scale by CPU and queue.media.transcode depth

delivery-worker:
  scale by queue.vk.delivery depth and VK rate limits
```

---

# 28. Репозиторий для Codex

Я бы делал monorepo на Go.

```text
ai-vk-aggregator/
  AGENTS.md
  README.md
  Makefile
  docker-compose.yml
  go.mod
  go.sum

  cmd/
    api/
    vk-inbound/
    worker/
    provider-webhook/
    admin-api/
    migrate/

  internal/
    app/
      api/
      worker/
      admin/

    domain/
      user/
      conversation/
      command/
      job/
      workflow/
      provider/
      artifact/
      billing/
      moderation/
      delivery/

    service/
      commandrouter/
      joborchestrator/
      providerrouting/
      billingservice/
      moderationservice/
      deliveryservice/
      mediaservice/

    adapter/
      inbound/
        vk/
      provider/
        openai/
        google/
        kling/
        mock/
      storage/
        postgres/
        redis/
        s3/
      queue/
        kafka/
        rabbitmq/
        nats/
      workflow/
        temporal/
      delivery/
        vk/

    platform/
      config/
      logger/
      metrics/
      tracing/
      errors/
      idempotency/
      outbox/
      ratelimit/
      security/

  migrations/
    001_init.sql
    002_jobs.sql
    003_billing.sql
    004_artifacts.sql

  api/
    openapi.yaml
    asyncapi.yaml

  docs/
    architecture.md
    adr/
      0001-use-go.md
      0002-use-postgres.md
      0003-use-temporal.md
      0004-provider-abstraction.md
      0005-billing-ledger.md

  deployments/
    docker/
    k8s/
    helm/

  test/
    integration/
    fixtures/
```

---

# 29. AGENTS.md для Codex

Codex нужно сразу ограничить архитектурными правилами. OpenAI в своих рекомендациях по Codex прямо советует давать задаче цель, контекст, ограничения и критерий готовности; для повторяемых правил используется `AGENTS.md`, куда стоит записывать layout, команды сборки/тестов, conventions и definition of done. ([OpenAI Разработчики][11])

Пример `AGENTS.md` для этого проекта:

```md
# AGENTS.md

## Project

This repository is a Go backend for a VK AI aggregator.
The system is an AI job processing platform, not a simple chatbot.

## Architecture rules

- Do not call AI providers directly from VK handlers.
- All user requests must become Jobs.
- All external inbound events must be idempotent.
- All provider calls must go through internal/adapter/provider.
- All VK API calls must go through internal/adapter/delivery/vk.
- Billing must use ledger entries and reservations; never mutate balance directly without ledger.
- Media files must be stored as Artifacts before delivery.
- Workers must be safe to retry.
- Provider adapters must not know about VK delivery or billing.
- Delivery service must not know provider-specific API details.
- Use context.Context for request-scoped cancellation and timeouts.
- Do not log secrets, tokens, raw provider keys, or full PII.

## Commands

- Run tests: go test ./...
- Run lint: golangci-lint run
- Format: gofmt -w .
- Run migrations locally: go run ./cmd/migrate up
- Run local stack: docker compose up -d

## Done means

- Code compiles.
- Tests pass.
- Public behavior is covered by tests.
- New DB changes include migrations.
- New provider adapters include mock tests.
- New workers are idempotent and retry-safe.
```

Codex также умеет читать `AGENTS.md` из глобального и project scope, а более близкие к текущей директории инструкции имеют больший приоритет; это удобно, если ты хочешь отдельные правила для `provider/kling`, `billing` или `delivery/vk`. ([OpenAI Разработчики][12])

---

# 30. Как дробить задачи для Codex

Не давай Codex задачу:

```text
Сделай мне весь агрегатор нейросетей.
```

Это плохой prompt.

Давай так:

```text
Задача 1:
Создай domain-модели Job, JobStatus, Artifact, ProviderTask.
Не подключай внешние API. Добавь unit tests.

Задача 2:
Реализуй Postgres repository для Job.
Используй транзакции. Добавь миграцию. Добавь integration tests.

Задача 3:
Реализуй idempotency service.
Нужны методы GetOrCreate, MarkCompleted, MarkFailed.

Задача 4:
Реализуй provider interface и mock provider.
Никаких OpenAI/Kling пока.

Задача 5:
Реализуй JobOrchestrator.CreateJob:
- validate command
- reserve credits через интерфейс Billing
- create job
- write outbox event
- return job id

Задача 6:
Реализуй VK inbound handler:
- parse raw event
- validate group
- save inbound event
- enqueue command
- return ok
```

Codex docs рекомендуют для сложных задач сначала просить план; там же описан Plan mode для сложных/неоднозначных изменений. Для такого проекта это критично: сначала план и affected files, потом код. ([OpenAI Разработчики][11])

---

# 31. Доменный контракт

Вот какие сущности должны быть центральными:

```text
User
Conversation
InboundEvent
Command
Job
WorkflowRun
ProviderTask
Artifact
Delivery
LedgerEntry
ModerationResult
OutboxEvent
```

Связи:

```text
InboundEvent -> Command -> Job -> ProviderTask -> Artifact -> Delivery
                         -> LedgerEntry
                         -> ModerationResult
                         -> OutboxEvent
```

Главный invariant:

```text
Пользовательский запрос не считается выполненным, пока:
  - job.status = succeeded;
  - output artifact сохранён;
  - delivery.status = sent;
  - billing capture выполнен;
  - audit event записан.
```

---

# 32. Внутренний API между сервисами

Даже если сначала будет один Go-монолит, контракты надо проектировать как service boundaries.

Примеры internal APIs:

```text
POST /internal/jobs
GET  /internal/jobs/{id}
POST /internal/jobs/{id}/cancel
POST /internal/jobs/{id}/retry

POST /internal/provider/submit
GET  /internal/provider/tasks/{id}
POST /internal/provider/webhooks/{provider}

POST /internal/artifacts
GET  /internal/artifacts/{id}/download-url

POST /internal/delivery/vk
GET  /internal/delivery/{id}

POST /internal/billing/reserve
POST /internal/billing/capture
POST /internal/billing/release

POST /internal/moderation/check
```

Для внешней админки:

```text
GET /admin/jobs
GET /admin/jobs/{id}
POST /admin/jobs/{id}/retry
POST /admin/jobs/{id}/refund
POST /admin/providers/{id}/disable
POST /admin/users/{id}/ban
```

---

# 33. Что должно быть в OpenAPI / AsyncAPI

## OpenAPI

```text
- admin API;
- internal service APIs;
- health endpoints;
- provider webhook endpoints;
- billing endpoints.
```

## AsyncAPI

```text
- event.job.created
- event.job.status_changed
- event.provider.task_completed
- event.artifact.created
- event.delivery.sent
- event.billing.captured
```

Codex будет гораздо лучше писать код, если контракты есть заранее.

---

# 34. Максимальный production stack

Я бы выбрал так:

```text
Language:
  Go

API:
  net/http + chi или connect-go/gRPC для internal

DB:
  PostgreSQL

Cache / locks / rate limit:
  Redis

Workflow:
  Temporal

Event bus:
  Kafka / Redpanda

Object storage:
  S3-compatible storage / MinIO

Analytics:
  ClickHouse

Search/logs:
  Loki или OpenSearch

Metrics:
  Prometheus + Grafana

Tracing:
  OpenTelemetry + Tempo/Jaeger

Deployment:
  Docker + Kubernetes + Helm

Secrets:
  Vault / cloud secret manager / sealed-secrets

CI/CD:
  GitHub Actions

Admin frontend:
  React + TypeScript

Media:
  ffmpeg workers

Optional Python:
  media/ML workers only, not core orchestration
```

---

# 35. Какие сервисы делать отдельными, а какие можно держать модулями

На старте можно держать в одном Go repo и даже в одном binary несколько режимов:

```text
go run ./cmd/api
go run ./cmd/worker --queue=image
go run ./cmd/worker --queue=video
go run ./cmd/provider-webhook
go run ./cmd/admin-api
```

Но архитектурно разделить boundaries сразу.

Физически отдельными сервисами в production:

```text
- vk-inbound-gateway
- job-orchestrator
- provider-gateway
- provider-webhook-receiver
- worker pools
- delivery-service
- admin-api
```

Можно оставить модулями внутри одного приложения:

```text
- prompt service
- command router
- pricing service
- model catalog
```

Пока они не стали большими.

---

# 36. Самые важные инварианты для Codex

Вот это я бы держал в `docs/architecture.md` и `AGENTS.md`.

```text
1. VK handlers never call providers.
2. Provider adapters never call VK.
3. Billing is append-only ledger.
4. Every external operation has idempotency key.
5. Every worker is retry-safe.
6. Every job status transition is explicit.
7. Every media file is an Artifact.
8. Every provider response is normalized.
9. Every delivery attempt is persisted.
10. Every webhook is deduplicated.
11. Every provider failure maps to internal error class.
12. Every long operation is asynchronous.
13. No raw secrets in logs.
14. No direct balance mutation without ledger entry.
15. No user output before moderation passes.
```

---

# 37. Что делать первым, но с максимальной архитектурой в голове

Не строить сразу 25 микросервисов. Строить **production-shaped modular monolith**:

```text
Phase 1:
  - Go monorepo
  - Postgres
  - Redis
  - S3/MinIO
  - VK inbound
  - Job model
  - Provider interface
  - Mock provider
  - VK delivery
  - Billing ledger skeleton

Phase 2:
  - OpenAI provider
  - Gemini/Nano Banana provider
  - image artifact pipeline
  - moderation
  - admin jobs page

Phase 3:
  - Kling provider
  - video workflow
  - polling/webhook receiver
  - ffmpeg transcode
  - delivery as VK video/doc

Phase 4:
  - Temporal
  - Kafka events
  - provider health/circuit breaker
  - ClickHouse analytics
  - advanced rate limits
  - k8s autoscaling

Phase 5:
  - multi-provider routing
  - fallback
  - A/B prompts
  - premium queues
  - subscriptions
  - full admin panel
```

То есть код можно начинать проще, но **границы должны быть как у production-системы**.

---

# 38. Итоговая архитектурная формула

Твой проект должен быть таким:

```text
VK AI Aggregator =
  VK Bot Gateway
  + AI Job Platform
  + Provider Gateway
  + Media Pipeline
  + Billing Ledger
  + Delivery System
  + Moderation Layer
  + Observability
  + Admin Control Plane
```

Самая важная мысль:

```text
Не "пользователь написал -> мы вызвали нейросеть".
А "пользователь создал управляемую, оплачиваемую, отслеживаемую, возобновляемую AI-задачу".
```

Для такого проекта **Go — правильный основной язык**. Python подключать точечно, когда реально понадобится ML/media tooling. Финальная production-архитектура должна строиться вокруг `Job`, `ProviderTask`, `Artifact`, `Delivery`, `LedgerEntry` и `WorkflowRun`.

[1]: https://platform.openai.com/docs/api-reference/responses "Responses | OpenAI API Reference"
[2]: https://platform.openai.com/docs/guides/webhooks "Webhooks | OpenAI API"
[3]: https://packagist.org/packages/vkcom/vk-php-sdk?utm_source=chatgpt.com "vkcom/vk-php-sdk"
[4]: https://ai.google.dev/gemini-api/docs/image-generation "Gemini API  |  Google AI for Developers"
[5]: https://docs.temporal.io/workflow-execution?utm_source=chatgpt.com "Temporal Workflow Execution overview"
[6]: https://platform.openai.com/docs/guides/rate-limits "Rate limits | OpenAI API"
[7]: https://raw.githubusercontent.com/VKCOM/vk-api-schema/master/messages/methods.json "raw.githubusercontent.com"
[8]: https://kafka.apache.org/documentation/?utm_source=chatgpt.com "Introduction | Apache Kafka"
[9]: https://opentelemetry.io/docs/?utm_source=chatgpt.com "Documentation"
[10]: https://kling.ai/document-api/apiReference/model/textToVideo?utm_source=chatgpt.com "Text-to-Video API"
[11]: https://developers.openai.com/codex/learn/best-practices "Best practices – Codex | OpenAI Developers"
[12]: https://developers.openai.com/codex/guides/agents-md "Custom instructions with AGENTS.md – Codex | OpenAI Developers"
