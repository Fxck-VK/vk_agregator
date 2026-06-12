package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
)

const vkTextChunkLimit = 3500

// ObjectStore fetches stored artifact bytes (needed to deliver text results).
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// URLSigner issues time-limited download URLs for stored artifacts so media is
// delivered via signed URLs rather than exposing the raw storage location
// (audit ST1).
type URLSigner interface {
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// DeliveryBiller captures a job's reserved credits once it is delivered.
type DeliveryBiller interface {
	CaptureForJob(ctx context.Context, jobID uuid.UUID, amount int64) error
}

// DeliveryDeps bundles the delivery worker's collaborators.
type DeliveryDeps struct {
	Jobs       domain.JobRepository
	Deliveries domain.DeliveryRepository
	Artifacts  domain.ArtifactRepository
	Objects    ObjectStore
	VK         vkdelivery.Client
	// VKControl edits control/product messages. If nil and VK also implements
	// vkdelivery.ControlClient, the worker uses VK as the control client.
	VKControl vkdelivery.ControlClient
	// VKUploader uploads raw photo/video artifact bytes to VK before send when
	// available. If nil and VK also implements vkdelivery.MediaUploader, the
	// worker uses VK as the uploader.
	VKUploader vkdelivery.MediaUploader
	Billing    DeliveryBiller
	// Streams, when set, receives dead-lettered delivery tasks once the retry
	// budget is exhausted.
	Streams StreamPublisher
	// MaxAttempts caps delivery send attempts before dead-lettering (default 3).
	MaxAttempts int
	// Backoff returns the delay before the next delivery retry; defaults to none.
	Backoff func(attempt int) time.Duration
	// Signer issues signed media URLs when SignedURLs is enabled (audit ST1).
	Signer URLSigner
	// SignedURLs delivers media via time-limited signed URLs instead of raw
	// bucket/key references.
	SignedURLs bool
	// RawVideoDeliveryPolicy controls when an original provider video may be
	// delivered without a VK-ready variant. Production should use
	// if_probe_passed or never.
	RawVideoDeliveryPolicy string
	// URLTTL is the validity window of signed media URLs (default 1h).
	URLTTL time.Duration
	Now    func() time.Time
}

// DeliveryWorker consumes the delivery stream and runs the final stage of the
// pipeline: Artifact -> Delivery -> Billing Capture -> Job Success. It is
// idempotent (one delivery row per job, deduped by key), uses a deterministic
// random_id so VK suppresses duplicate sends, and is safe to retry/recover.
type DeliveryWorker struct {
	jobs           domain.JobRepository
	deliveries     domain.DeliveryRepository
	artifacts      domain.ArtifactRepository
	objects        ObjectStore
	vk             vkdelivery.Client
	vkControl      vkdelivery.ControlClient
	vkUploader     vkdelivery.MediaUploader
	billing        DeliveryBiller
	streams        StreamPublisher
	maxAttempts    int
	backoff        func(attempt int) time.Duration
	signer         URLSigner
	signURLs       bool
	rawVideoPolicy string
	urlTTL         time.Duration
	now            func() time.Time
}

// NewDeliveryWorker builds a DeliveryWorker.
func NewDeliveryWorker(d DeliveryDeps) *DeliveryWorker {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = maxProviderAttempts
	}
	backoff := d.Backoff
	if backoff == nil {
		backoff = func(int) time.Duration { return 0 }
	}
	urlTTL := d.URLTTL
	if urlTTL <= 0 {
		urlTTL = time.Hour
	}
	uploader := d.VKUploader
	if uploader == nil {
		if up, ok := d.VK.(vkdelivery.MediaUploader); ok {
			uploader = up
		}
	}
	control := d.VKControl
	if control == nil {
		if c, ok := d.VK.(vkdelivery.ControlClient); ok {
			control = c
		}
	}
	return &DeliveryWorker{
		jobs:           d.Jobs,
		deliveries:     d.Deliveries,
		artifacts:      d.Artifacts,
		objects:        d.Objects,
		vk:             d.VK,
		vkControl:      control,
		vkUploader:     uploader,
		billing:        d.Billing,
		streams:        d.Streams,
		maxAttempts:    maxAttempts,
		backoff:        backoff,
		signer:         d.Signer,
		signURLs:       d.SignedURLs,
		rawVideoPolicy: rawVideoDeliveryPolicyOrDefault(d.RawVideoDeliveryPolicy),
		urlTTL:         urlTTL,
		now:            now,
	}
}

func rawVideoDeliveryPolicyOrDefault(policy string) string {
	policy = normalizeWorkerPolicy(policy)
	if policy == "" {
		return rawProviderVideoPolicyAlwaysDevOnly
	}
	return policy
}

// Process delivers one job's result. Returning nil acknowledges the task;
// returning an error leaves it pending for retry/recovery.
func (w *DeliveryWorker) Process(ctx context.Context, task queue.Task) error {
	ctx, span := tracing.Start(ctx, "delivery.process",
		attribute.String("job.id", task.JobID.String()),
		attribute.String("operation", string(task.Operation)),
		tracing.CorrelationAttr(task.CorrelationID),
	)
	defer span.End()

	job, err := w.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}
	span.SetAttributes(attribute.String("job.status", string(job.Status)))
	failureNotice := isTerminalImageFailureNotice(job)
	switch job.Status {
	case domain.JobStatusSucceeded:
		return nil
	case domain.JobStatusResultReady, domain.JobStatusDelivering:
		// deliverable
	case domain.JobStatusFailedTerminal:
		if !failureNotice {
			return nil
		}
	default:
		// Not ready to deliver yet (or in a failed/terminal state): ack and drop.
		return nil
	}

	if !failureNotice {
		if err := w.setStatus(ctx, job, domain.JobStatusDelivering, "", ""); err != nil {
			tracing.RecordError(span, err)
			return err
		}
	}

	del, err := w.ensureDelivery(ctx, job)
	if err != nil {
		tracing.RecordError(span, err)
		return err
	}

	if del.Status != domain.DeliveryStatusSent {
		if err := w.send(ctx, del, job); err != nil {
			tracing.RecordError(span, err)
			del.Status = domain.DeliveryStatusRetrying
			del.ErrorMessage = err.Error()
			del.AttemptNo++
			_ = w.deliveries.Update(ctx, del)
			// Retry budget: dead-letter once exhausted so a permanently failing
			// VK send can no longer be retried forever.
			if del.AttemptNo > w.maxAttempts {
				metrics.DLQRouted.WithLabelValues("delivery").Inc()
				if w.streams != nil {
					_ = w.streams.PublishTo(ctx, redisqueue.StreamDLQ, task)
				}
				metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusFailedTerminal)).Inc()
				return w.setStatus(ctx, job, domain.JobStatusFailedTerminal, "delivery_failed", err.Error())
			}
			w.sleepBackoff(ctx, del.AttemptNo)
			return fmt.Errorf("worker: vk send: %w", err)
		}
	}

	if failureNotice {
		metrics.DeliveriesSent.Inc()
		return nil
	}

	// Billing capture: charge the reserved credits now that delivery succeeded.
	if job.CostReserved > 0 {
		captureCtx, captureSpan := tracing.Start(ctx, "billing.capture",
			attribute.String("job.id", job.ID.String()),
			attribute.Int64("billing.amount", job.CostReserved),
			tracing.CorrelationAttr(job.CorrelationID),
		)
		if err := w.billing.CaptureForJob(captureCtx, job.ID, job.CostReserved); err != nil {
			metrics.BillingCaptures.WithLabelValues(deliveryOperationLabel(job), "error").Inc()
			tracing.RecordError(captureSpan, err)
			captureSpan.End()
			tracing.RecordError(span, err)
			return fmt.Errorf("worker: capture: %w", err)
		}
		metrics.BillingCaptures.WithLabelValues(deliveryOperationLabel(job), "success").Inc()
		metrics.AddProductCreditsFlow("job_delivery", "capture", "success", job.CostReserved)
		captureSpan.End()
		if job.CostCaptured != job.CostReserved {
			job.CostCaptured = job.CostReserved
			if err := w.jobs.Update(ctx, job); err != nil {
				tracing.RecordError(span, err)
				return err
			}
		}
	}

	metrics.DeliveriesSent.Inc()
	metrics.JobsTerminal.WithLabelValues(string(domain.JobStatusSucceeded)).Inc()
	return w.setStatus(ctx, job, domain.JobStatusSucceeded, "", "")
}

// ensureDelivery returns the job's delivery row, creating it on first run. The
// delivery is keyed by job so a retry reuses the same row and random_id.
func (w *DeliveryWorker) ensureDelivery(ctx context.Context, job *domain.Job) (*domain.Delivery, error) {
	key := "delivery:" + job.ID.String()
	if existing, err := w.deliveries.GetByIdempotencyKey(ctx, key); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	del, err := w.buildDelivery(ctx, job, key)
	if err != nil {
		return nil, err
	}
	if err := w.deliveries.Create(ctx, del); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return w.deliveries.GetByIdempotencyKey(ctx, key)
		}
		return nil, err
	}
	return del, nil
}

// buildDelivery assembles a pending delivery from the job's output artifact.
func (w *DeliveryWorker) buildDelivery(ctx context.Context, job *domain.Job, key string) (*domain.Delivery, error) {
	var params promptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &params)
	}

	del := &domain.Delivery{
		JobID:          job.ID,
		UserID:         job.UserID,
		VKPeerID:       job.VKPeerID,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     vkdelivery.DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
	}

	if isTerminalImageFailureNotice(job) {
		del.Text = "Не удалось сгенерировать изображение. Средства не списаны. Попробуйте позже или измените описание."
		if params.VKPlaceholderMessageID > 0 {
			msgID := params.VKPlaceholderMessageID
			del.VKMessageID = &msgID
		}
		return del, nil
	}

	if len(job.OutputArtifactIDs) == 0 {
		del.Text = "(no result produced)"
		return del, nil
	}

	artID := job.OutputArtifactIDs[0]
	art, err := w.artifacts.GetByID(ctx, artID)
	if err != nil {
		return nil, err
	}
	del.ArtifactID = &artID

	switch art.MediaType {
	case domain.MediaTypeImage:
		del.Type = domain.DeliveryTypePhoto
	case domain.MediaTypeVideo:
		del.Type = domain.DeliveryTypeVideo
	default:
		del.Type = domain.DeliveryTypeMessage
		del.Text = w.textContent(ctx, art)
		if params.VKPlaceholderMessageID > 0 {
			msgID := params.VKPlaceholderMessageID
			del.VKMessageID = &msgID
		}
	}
	return del, nil
}

func isTerminalImageFailureNotice(job *domain.Job) bool {
	return job.Status == domain.JobStatusFailedTerminal &&
		job.VKPeerID != 0 &&
		job.Modality == domain.ModalityImage
}

// textContent loads the stored text bytes for a text artifact, falling back to
// a placeholder when the bytes are unavailable.
func (w *DeliveryWorker) textContent(ctx context.Context, art *domain.Artifact) string {
	if w.objects == nil {
		return "(result ready)"
	}
	data, err := w.objects.GetObject(ctx, art.StorageBucket, art.StorageKey)
	if err != nil {
		return "(result ready)"
	}
	return formatVKText(string(data))
}

func (w *DeliveryWorker) send(ctx context.Context, del *domain.Delivery, job *domain.Job) error {
	ctx, span := tracing.Start(ctx, "vk.delivery.send",
		attribute.String("delivery.id", del.ID.String()),
		attribute.String("delivery.type", string(del.Type)),
		attribute.Int64("vk.peer_id", del.VKPeerID),
	)
	defer span.End()

	started := time.Now()
	kind := deliveryKind(del.Type)
	var (
		res vkdelivery.SendResult
		err error
	)
	switch del.Type {
	case domain.DeliveryTypePhoto:
		if err := w.ensureMediaAttachment(ctx, del, job); err != nil {
			class := deliveryErrorClass(err)
			metrics.VKUploadFailures.WithLabelValues("image", class).Inc()
			metrics.VKDeliveryAttempts.WithLabelValues(kind, "error", class).Inc()
			metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
			return err
		}
		res, err = w.vk.SendPhoto(ctx, del.VKPeerID, del.VKRandomID, del.Attachment, del.Text)
	case domain.DeliveryTypeVideo:
		if err := w.ensureMediaAttachment(ctx, del, job); err != nil {
			class := deliveryErrorClass(err)
			metrics.VKUploadFailures.WithLabelValues("video", class).Inc()
			metrics.VKDeliveryAttempts.WithLabelValues(kind, "error", class).Inc()
			metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
			return err
		}
		res, err = w.vk.SendVideo(ctx, del.VKPeerID, del.VKRandomID, del.Attachment, del.Text)
	default:
		res, err = w.sendTextDelivery(ctx, del)
	}
	if err != nil {
		tracing.RecordError(span, err)
		class := deliveryErrorClass(err)
		metrics.VKDeliveryAttempts.WithLabelValues(kind, "error", class).Inc()
		metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
		return err
	}
	metrics.VKDeliveryAttempts.WithLabelValues(kind, "success", "").Inc()
	metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
	msgID := res.MessageID
	span.SetAttributes(attribute.Int64("vk.message_id", msgID))
	del.Status = domain.DeliveryStatusSent
	del.VKMessageID = &msgID
	del.ErrorCode = ""
	del.ErrorMessage = ""
	return w.deliveries.Update(ctx, del)
}

func (w *DeliveryWorker) sendTextDelivery(ctx context.Context, del *domain.Delivery) (vkdelivery.SendResult, error) {
	chunks := splitVKText(del.Text)
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	var first vkdelivery.SendResult
	var err error
	if del.VKMessageID != nil && *del.VKMessageID > 0 && w.vkControl != nil {
		first, err = w.vkControl.EditMessage(ctx, del.VKPeerID, *del.VKMessageID, vkdelivery.Message{Text: chunks[0]})
	} else {
		first, err = w.vk.SendText(ctx, del.VKPeerID, del.VKRandomID, chunks[0])
	}
	if err != nil {
		return vkdelivery.SendResult{}, err
	}

	for i := 1; i < len(chunks); i++ {
		randomID := vkdelivery.DeterministicRandomID(del.IdempotencyKey + ":chunk:" + strconv.Itoa(i))
		if _, err := w.vk.SendText(ctx, del.VKPeerID, randomID, chunks[i]); err != nil {
			return vkdelivery.SendResult{}, err
		}
	}
	return first, nil
}

func (w *DeliveryWorker) ensureMediaAttachment(ctx context.Context, del *domain.Delivery, job *domain.Job) error {
	if del.Attachment != "" {
		return nil
	}
	if del.ArtifactID == nil {
		return fmt.Errorf("worker: media delivery has no artifact")
	}
	art, err := w.artifacts.GetByID(ctx, *del.ArtifactID)
	if err != nil {
		return err
	}
	attachment, err := w.mediaAttachment(ctx, del.VKPeerID, art, promptFromJob(job))
	if err != nil {
		return err
	}
	del.Attachment = attachment
	return w.deliveries.Update(ctx, del)
}

func splitVKText(text string) []string {
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= vkTextChunkLimit {
		return []string{text}
	}

	var chunks []string
	for len(runes) > 0 {
		n := vkTextChunkLimit
		if len(runes) < n {
			n = len(runes)
		}
		cut := n
		for i := n - 1; i > 0; i-- {
			switch runes[i] {
			case '\n', ' ', '\t':
				cut = i + 1
				i = 0
			}
		}
		chunk := strings.TrimSpace(string(runes[:cut]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		runes = runes[cut:]
	}
	return chunks
}

func formatVKText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = formatVKLine(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatVKLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	for strings.HasPrefix(trimmed, "#") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
	}

	if rest, ok := markdownBulletRest(trimmed); ok {
		return "• " + stripVKMarkdown(rest)
	}

	return stripVKMarkdown(trimmed)
}

func markdownBulletRest(line string) (string, bool) {
	for _, marker := range []string{"* ", "*\t", "- ", "-\t"} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker)), true
		}
	}
	return "", false
}

func stripVKMarkdown(text string) string {
	for _, marker := range []string{"**", "__", "`", "*"} {
		text = strings.ReplaceAll(text, marker, "")
	}
	return strings.TrimSpace(text)
}

// sleepBackoff waits for the configured backoff before the next retry, honoring
// context cancellation.
func (w *DeliveryWorker) sleepBackoff(ctx context.Context, attempt int) {
	d := w.backoff(attempt)
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (w *DeliveryWorker) setStatus(ctx context.Context, job *domain.Job, to domain.JobStatus, code, msg string) error {
	if job.Status == to {
		return nil
	}
	from := job.Status
	if err := w.jobs.UpdateStatus(ctx, job.ID, job.Status, to, code, msg); err != nil {
		return err
	}
	job.Status = to
	metrics.JobStatusCurrent.WithLabelValues(string(from), deliveryOperationLabel(job), deliveryModalityLabel(job)).Dec()
	metrics.JobStatusCurrent.WithLabelValues(string(to), deliveryOperationLabel(job), deliveryModalityLabel(job)).Inc()
	if to.IsTerminal() && !job.CreatedAt.IsZero() {
		duration := time.Since(job.CreatedAt)
		if duration > 0 {
			metrics.JobDuration.WithLabelValues(deliveryOperationLabel(job), deliveryModalityLabel(job), string(to)).Observe(duration.Seconds())
		}
	}
	if to.IsTerminal() {
		metrics.ObserveProductEvent("worker", "job", "terminal", deliveryOperationLabel(job), deliveryModalityLabel(job), string(to))
	}
	return nil
}

// mediaAttachment resolves the attachment reference for a media artifact. With
// a real VK uploader and stored bytes, it uploads the selected media object to VK
// and returns the VK attachment string. For videos, a ready VK-specific variant
// is preferred over raw provider output. Otherwise, when signed delivery is
// enabled it issues a time-limited signed URL; finally it falls back to the
// selected object's public URL or storage location.
func (w *DeliveryWorker) mediaAttachment(ctx context.Context, peerID int64, art *domain.Artifact, filenamePrompt string) (string, error) {
	if ref := attachmentRef(art); isVKAttachment(ref) {
		return ref, nil
	}
	obj, err := w.mediaObjectForDelivery(ctx, art)
	if err != nil {
		return "", err
	}
	if w.vkUploader != nil && w.objects != nil && obj.storageKey != "" {
		data, err := w.objects.GetObject(ctx, obj.storageBucket, obj.storageKey)
		if err != nil {
			return "", fmt.Errorf("worker: load artifact for vk upload: %w", err)
		}
		name := artifactFilename(art, filenamePrompt)
		switch art.MediaType {
		case domain.MediaTypeImage:
			return w.vkUploader.UploadPhoto(ctx, peerID, name, data, obj.mimeType)
		case domain.MediaTypeVideo:
			return w.vkUploader.UploadVideo(ctx, peerID, name, data, obj.mimeType)
		}
	}
	if w.signURLs && w.signer != nil && obj.storageKey != "" {
		if signed, err := w.signer.PresignedGetURL(ctx, obj.storageBucket, obj.storageKey, w.urlTTL); err == nil && signed != "" {
			return signed, nil
		}
	}
	if obj.fallbackRef != "" {
		return obj.fallbackRef, nil
	}
	return attachmentRef(art), nil
}

type mediaDeliveryObject struct {
	storageBucket string
	storageKey    string
	mimeType      string
	fallbackRef   string
}

func (w *DeliveryWorker) mediaObjectForDelivery(ctx context.Context, art *domain.Artifact) (mediaDeliveryObject, error) {
	obj := mediaDeliveryObject{
		storageBucket: art.StorageBucket,
		storageKey:    art.StorageKey,
		mimeType:      art.MimeType,
		fallbackRef:   attachmentRef(art),
	}
	if art.MediaType != domain.MediaTypeVideo {
		return obj, nil
	}
	variants, err := w.artifacts.ListVariants(ctx, art.ID)
	if err != nil {
		return obj, fmt.Errorf("worker: list artifact variants: %w", err)
	}
	for _, variantType := range []domain.VariantType{domain.VariantVKDoc, domain.VariantVKVideo} {
		for _, variant := range variants {
			if !readyVideoVariant(variant, variantType) {
				continue
			}
			mimeType := variant.MimeType
			if mimeType == "" {
				mimeType = "video/mp4"
			}
			return mediaDeliveryObject{
				storageBucket: variant.StorageBucket,
				storageKey:    variant.StorageKey,
				mimeType:      mimeType,
				fallbackRef:   variant.StorageBucket + "/" + variant.StorageKey,
			}, nil
		}
	}
	if readyOriginalVideo(art, w.rawVideoPolicy) {
		return obj, nil
	}
	return obj, fmt.Errorf("worker: video original is not allowed for delivery without ready variant")
}

func readyOriginalVideo(art *domain.Artifact, policy string) bool {
	if art == nil || art.MediaType != domain.MediaTypeVideo || art.StorageBucket == "" || art.StorageKey == "" {
		return false
	}
	switch normalizeWorkerPolicy(policy) {
	case rawProviderVideoPolicyAlwaysDevOnly:
		return true
	case rawProviderVideoPolicyIfProbePassed:
		return art.ProbeStatus == domain.MediaProbePassed &&
			strings.EqualFold(art.Container, "mp4") &&
			strings.EqualFold(art.Codec, "h264")
	default:
		return false
	}
}

func readyVideoVariant(variant *domain.ArtifactVariant, variantType domain.VariantType) bool {
	return variant != nil &&
		variant.VariantType == variantType &&
		variant.StorageBucket != "" &&
		variant.StorageKey != "" &&
		variant.ProbeStatus == domain.MediaProbePassed
}

// attachmentRef returns the VK attachment reference for a media artifact,
// preferring a public URL and falling back to the storage location.
func attachmentRef(art *domain.Artifact) string {
	if art.PublicURL != "" {
		return art.PublicURL
	}
	return art.StorageBucket + "/" + art.StorageKey
}

func isVKAttachment(ref string) bool {
	return strings.HasPrefix(ref, "photo") || strings.HasPrefix(ref, "video") || strings.HasPrefix(ref, "doc")
}

func deliveryKind(t domain.DeliveryType) string {
	switch t {
	case domain.DeliveryTypePhoto:
		return "photo"
	case domain.DeliveryTypeVideo:
		return "video"
	default:
		return "text"
	}
}

func deliveryErrorClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	value := strings.ToLower(err.Error())
	switch {
	case strings.Contains(value, "rate"):
		return "rate_limited"
	case strings.Contains(value, "http 4"), strings.Contains(value, "vk error"):
		return "vk_error"
	case strings.Contains(value, "http 5"):
		return "upstream_error"
	case strings.Contains(value, "upload"):
		return "upload_error"
	case strings.Contains(value, "storage"), strings.Contains(value, "artifact"):
		return "artifact_error"
	default:
		return "internal_error"
	}
}

func deliveryOperationLabel(job *domain.Job) string {
	if job == nil || job.OperationType == "" {
		return "unknown"
	}
	return deliveryMetricLabel(string(job.OperationType))
}

func deliveryModalityLabel(job *domain.Job) string {
	if job == nil || job.Modality == "" {
		return "unknown"
	}
	return deliveryMetricLabel(string(job.Modality))
}

func deliveryMetricLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == ':' || r == '/':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

func promptFromJob(job *domain.Job) string {
	if job == nil || len(job.Params) == 0 {
		return ""
	}
	var params promptParams
	if err := json.Unmarshal(job.Params, &params); err != nil {
		return ""
	}
	return params.Prompt
}

func artifactFilename(art *domain.Artifact, prompt string) string {
	ext := "bin"
	switch art.MediaType {
	case domain.MediaTypeImage:
		ext = "png"
	case domain.MediaTypeVideo:
		ext = "mp4"
	}
	if base := promptFilenameBase(prompt, 25); base != "" {
		return base + "." + ext
	}
	return art.ID.String() + "." + ext
}

func promptFilenameBase(prompt string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	normalized := strings.Join(strings.Fields(prompt), " ")
	if normalized == "" {
		return ""
	}
	out := make([]rune, 0, maxRunes)
	for _, r := range normalized {
		if len(out) >= maxRunes {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			continue
		default:
			out = append(out, r)
		}
	}
	return strings.Trim(strings.TrimSpace(string(out)), ".")
}
