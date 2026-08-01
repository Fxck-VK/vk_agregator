package vkdelivery

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

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
)

const vkTextChunkLimit = 3500

// ObjectStore fetches stored artifact bytes for text and media publication.
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// URLSigner issues a bounded download URL when signed-link delivery is enabled.
type URLSigner interface {
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// PublisherDeps contains the VK publisher's channel-specific dependencies.
type PublisherDeps struct {
	Deliveries             domain.DeliveryRepository
	Artifacts              domain.ArtifactRepository
	Objects                ObjectStore
	Client                 Client
	Control                ControlClient
	Uploader               MediaUploader
	Signer                 URLSigner
	SignedURLs             bool
	RawVideoDeliveryPolicy string
	URLTTL                 time.Duration
}

// Publisher implements external result publication for VK Bot targets.
type Publisher struct {
	deliveries     domain.DeliveryRepository
	artifacts      domain.ArtifactRepository
	objects        ObjectStore
	client         Client
	control        ControlClient
	uploader       MediaUploader
	signer         URLSigner
	signedURLs     bool
	rawVideoPolicy string
	urlTTL         time.Duration
}

// NewPublisher constructs a VK publisher. Client capabilities are reused for
// control-message edits and uploads when explicit collaborators are omitted.
func NewPublisher(deps PublisherDeps) *Publisher {
	control := deps.Control
	if control == nil {
		if candidate, ok := deps.Client.(ControlClient); ok {
			control = candidate
		}
	}
	uploader := deps.Uploader
	if uploader == nil {
		if candidate, ok := deps.Client.(MediaUploader); ok {
			uploader = candidate
		}
	}
	urlTTL := deps.URLTTL
	if urlTTL <= 0 {
		urlTTL = time.Hour
	}
	rawPolicy := strings.ToLower(strings.TrimSpace(deps.RawVideoDeliveryPolicy))
	if rawPolicy == "" {
		rawPolicy = "always_dev_only"
	}
	return &Publisher{
		deliveries:     deps.Deliveries,
		artifacts:      deps.Artifacts,
		objects:        deps.Objects,
		client:         deps.Client,
		control:        control,
		uploader:       uploader,
		signer:         deps.Signer,
		signedURLs:     deps.SignedURLs,
		rawVideoPolicy: rawPolicy,
		urlTTL:         urlTTL,
	}
}

// Channel identifies the exact external target handled by this publisher.
func (*Publisher) Channel() domain.Channel {
	return domain.ChannelVKBot
}

// BuildDelivery validates the authoritative VK target and creates the pending,
// deterministic delivery representation. It does not persist or publish it.
func (p *Publisher) BuildDelivery(ctx context.Context, job *domain.Job, idempotencyKey string) (*domain.Delivery, error) {
	peerID, target, err := validatedVKTarget(job)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("%w: empty delivery idempotency key", domain.ErrInvalidResultContract)
	}
	if p.deliveries == nil {
		return nil, errors.New("vkdelivery: delivery repository is not configured")
	}
	failureNotice := terminalMediaFailure(job)
	if !failureNotice && len(job.OutputArtifactIDs) == 0 {
		return nil, fmt.Errorf("%w: successful external result has no output artifacts", domain.ErrInvalidResultContract)
	}
	existing, err := p.deliveries.GetByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		if err := p.validateReplay(ctx, job, existing, peerID, target); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("vkdelivery: load delivery replay: %w", err)
	}

	var params publisherPromptParams
	if len(job.Params) > 0 {
		_ = json.Unmarshal(job.Params, &params)
	}
	delivery := &domain.Delivery{
		JobID:          job.ID,
		UserID:         job.UserID,
		AccountID:      job.AccountID,
		VKPeerID:       peerID,
		Target:         target,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     DeterministicRandomID(idempotencyKey),
		IdempotencyKey: idempotencyKey,
		AttemptNo:      1,
	}

	if failureNotice {
		delivery.Text = safeVKMediaFailureNotice(job.ErrorCode)
		if params.VKPlaceholderMessageID > 0 {
			messageID := params.VKPlaceholderMessageID
			delivery.VKMessageID = &messageID
		}
		return delivery, nil
	}

	artifactID := job.OutputArtifactIDs[0]
	artifact, err := p.outputArtifact(ctx, job, artifactID)
	if err != nil {
		return nil, err
	}
	delivery.ArtifactID = &artifactID
	switch artifact.MediaType {
	case domain.MediaTypeImage:
		delivery.Type = domain.DeliveryTypePhoto
	case domain.MediaTypeVideo:
		delivery.Type = domain.DeliveryTypeVideo
	default:
		delivery.Text = p.textContent(ctx, artifact)
		if params.VKPlaceholderMessageID > 0 {
			messageID := params.VKPlaceholderMessageID
			delivery.VKMessageID = &messageID
		}
	}
	return delivery, nil
}

// Publish validates replay target identity, backfills a safe legacy target,
// publishes once, and persists the confirmed send.
func (p *Publisher) Publish(ctx context.Context, job *domain.Job, delivery *domain.Delivery) error {
	peerID, target, err := validatedVKTarget(job)
	if err != nil {
		return err
	}
	if delivery == nil || delivery.JobID != job.ID {
		return fmt.Errorf("%w: delivery does not belong to job", domain.ErrInvalidResultContract)
	}
	if err := p.validateReplay(ctx, job, delivery, peerID, target); err != nil {
		return err
	}
	if delivery.Status == domain.DeliveryStatusSent {
		return nil
	}
	return p.send(ctx, job, delivery, peerID)
}

func (p *Publisher) validateReplay(
	ctx context.Context,
	job *domain.Job,
	delivery *domain.Delivery,
	peerID int64,
	target *domain.DeliveryTarget,
) error {
	if delivery == nil || delivery.JobID != job.ID {
		return fmt.Errorf("%w: delivery does not belong to job", domain.ErrInvalidResultContract)
	}
	if delivery.AccountID != uuid.Nil && job.AccountID != uuid.Nil && delivery.AccountID != job.AccountID {
		return fmt.Errorf("%w: delivery account does not match job owner", domain.ErrInvalidResultContract)
	}
	if delivery.Target != nil && !sameTarget(delivery.Target, target) {
		return fmt.Errorf("%w: delivery target does not match job target", domain.ErrInvalidResultContract)
	}
	if delivery.VKPeerID != 0 && delivery.VKPeerID != peerID {
		return fmt.Errorf("%w: persisted VK peer does not match job target", domain.ErrInvalidResultContract)
	}
	if delivery.Target == nil {
		if delivery.VKPeerID != peerID {
			return fmt.Errorf("%w: legacy delivery peer does not match job target", domain.ErrInvalidResultContract)
		}
		delivery.Target = target
		if delivery.AccountID == uuid.Nil {
			delivery.AccountID = job.AccountID
		}
		if err := p.updateDelivery(ctx, delivery); err != nil {
			return fmt.Errorf("vkdelivery: backfill delivery target: %w", err)
		}
	}
	return nil
}

func validatedVKTarget(job *domain.Job) (int64, *domain.DeliveryTarget, error) {
	if job == nil || job.ResultMode != domain.ResultModeExternalPush || job.DeliveryTarget == nil {
		return 0, nil, fmt.Errorf("%w: VK publication requires external-push target", domain.ErrInvalidResultContract)
	}
	if err := job.ValidateResultContract(); err != nil {
		return 0, nil, err
	}
	if job.DeliveryTarget.Channel != domain.ChannelVKBot {
		return 0, nil, fmt.Errorf("%w: unsupported VK target channel %q", domain.ErrInvalidResultContract, job.DeliveryTarget.Channel)
	}
	recipient := strings.TrimSpace(job.DeliveryTarget.RecipientRef)
	peerID, err := strconv.ParseInt(recipient, 10, 64)
	if err != nil || peerID <= 0 || strconv.FormatInt(peerID, 10) != recipient {
		return 0, nil, fmt.Errorf("%w: malformed VK recipient reference", domain.ErrInvalidResultContract)
	}
	target := *job.DeliveryTarget
	return peerID, &target, nil
}

func sameTarget(left, right *domain.DeliveryTarget) bool {
	return left != nil &&
		right != nil &&
		left.Channel == right.Channel &&
		left.RecipientRef == right.RecipientRef &&
		left.ThreadRef == right.ThreadRef
}

func terminalMediaFailure(job *domain.Job) bool {
	return job != nil &&
		job.Status == domain.JobStatusFailedTerminal &&
		(job.Modality == domain.ModalityImage || job.Modality == domain.ModalityVideo)
}

func (p *Publisher) outputArtifact(ctx context.Context, job *domain.Job, artifactID uuid.UUID) (*domain.Artifact, error) {
	if p.artifacts == nil {
		return nil, errors.New("vkdelivery: artifact repository is not configured")
	}
	var (
		artifact *domain.Artifact
		err      error
	)
	if job.AccountID != uuid.Nil {
		artifact, err = p.artifacts.GetByIDForAccount(ctx, job.AccountID, artifactID)
	} else {
		artifact, err = p.artifacts.GetByID(ctx, artifactID)
	}
	if err != nil {
		return nil, fmt.Errorf("vkdelivery: load output artifact: %w", err)
	}
	if artifact.Kind != domain.ArtifactKindOutput ||
		artifact.Status != domain.ArtifactStatusReady ||
		artifact.JobID == nil ||
		*artifact.JobID != job.ID {
		return nil, errors.New("vkdelivery: artifact is not a ready linked output")
	}
	if job.AccountID == uuid.Nil && job.UserID != uuid.Nil && artifact.OwnerUserID != job.UserID {
		return nil, errors.New("vkdelivery: legacy artifact owner mismatch")
	}
	return artifact, nil
}

func (p *Publisher) textContent(ctx context.Context, artifact *domain.Artifact) string {
	if p.objects == nil {
		return "(result ready)"
	}
	data, err := p.objects.GetObject(ctx, artifact.StorageBucket, artifact.StorageKey)
	if err != nil {
		return "(result ready)"
	}
	return formatVKText(string(data))
}

func (p *Publisher) send(ctx context.Context, job *domain.Job, delivery *domain.Delivery, peerID int64) error {
	if p.client == nil {
		return errors.New("vkdelivery: client is not configured")
	}
	ctx, span := tracing.Start(ctx, "vk.delivery.send",
		attribute.String("delivery.id", delivery.ID.String()),
		attribute.String("delivery.type", string(delivery.Type)),
		attribute.Int64("vk.peer_id", peerID),
	)
	defer span.End()

	started := time.Now()
	kind := deliveryKind(delivery.Type)
	var (
		result SendResult
		err    error
	)
	switch delivery.Type {
	case domain.DeliveryTypePhoto:
		if err := p.ensureMediaAttachment(ctx, job, delivery, peerID); err != nil {
			class := deliveryErrorClass(err)
			metrics.VKUploadFailures.WithLabelValues("image", class).Inc()
			metrics.VKDeliveryAttempts.WithLabelValues(kind, "error", class).Inc()
			metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
			return err
		}
		if p.control != nil {
			result, err = p.control.SendMessage(ctx, peerID, delivery.VKRandomID, Message{
				Text:       delivery.Text,
				Attachment: delivery.Attachment,
				Keyboard:   imageResultKeyboard(),
			})
		} else {
			result, err = p.client.SendPhoto(ctx, peerID, delivery.VKRandomID, delivery.Attachment, delivery.Text)
		}
	case domain.DeliveryTypeVideo:
		if err := p.ensureMediaAttachment(ctx, job, delivery, peerID); err != nil {
			class := deliveryErrorClass(err)
			metrics.VKUploadFailures.WithLabelValues("video", class).Inc()
			metrics.VKDeliveryAttempts.WithLabelValues(kind, "error", class).Inc()
			metrics.VKDeliveryDuration.WithLabelValues(kind).Observe(time.Since(started).Seconds())
			return err
		}
		result, err = p.client.SendVideo(ctx, peerID, delivery.VKRandomID, delivery.Attachment, delivery.Text)
	default:
		result, err = p.sendTextDelivery(ctx, delivery, peerID)
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
	messageID := result.MessageID
	span.SetAttributes(attribute.Int64("vk.message_id", messageID))
	delivery.Status = domain.DeliveryStatusSent
	delivery.VKMessageID = &messageID
	delivery.ErrorCode = ""
	delivery.ErrorMessage = ""
	return p.updateDelivery(ctx, delivery)
}

func (p *Publisher) sendTextDelivery(ctx context.Context, delivery *domain.Delivery, peerID int64) (SendResult, error) {
	chunks := splitVKText(delivery.Text)
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	var (
		first SendResult
		err   error
	)
	if delivery.VKMessageID != nil && *delivery.VKMessageID > 0 && p.control != nil {
		first, err = p.control.EditMessage(ctx, peerID, *delivery.VKMessageID, Message{Text: chunks[0]})
	} else {
		first, err = p.client.SendText(ctx, peerID, delivery.VKRandomID, chunks[0])
	}
	if err != nil {
		return SendResult{}, err
	}
	for index := 1; index < len(chunks); index++ {
		randomID := DeterministicRandomID(delivery.IdempotencyKey + ":chunk:" + strconv.Itoa(index))
		if _, err := p.client.SendText(ctx, peerID, randomID, chunks[index]); err != nil {
			return SendResult{}, err
		}
	}
	return first, nil
}

func (p *Publisher) ensureMediaAttachment(ctx context.Context, job *domain.Job, delivery *domain.Delivery, peerID int64) error {
	if delivery.Attachment != "" {
		return nil
	}
	if delivery.ArtifactID == nil {
		return errors.New("vkdelivery: media delivery has no artifact")
	}
	artifact, err := p.outputArtifact(ctx, job, *delivery.ArtifactID)
	if err != nil {
		return err
	}
	attachment, err := p.mediaAttachment(ctx, peerID, artifact, promptFromJob(job))
	if err != nil {
		return err
	}
	delivery.Attachment = attachment
	return p.updateDelivery(ctx, delivery)
}

func (p *Publisher) updateDelivery(ctx context.Context, delivery *domain.Delivery) error {
	if p.deliveries == nil {
		return errors.New("vkdelivery: delivery repository is not configured")
	}
	return p.deliveries.Update(ctx, delivery)
}

func (p *Publisher) mediaAttachment(ctx context.Context, peerID int64, artifact *domain.Artifact, filenamePrompt string) (string, error) {
	if reference := attachmentRef(artifact); isVKAttachment(reference) {
		return reference, nil
	}
	object, err := p.mediaObjectForDelivery(ctx, artifact)
	if err != nil {
		return "", err
	}
	if p.uploader != nil && p.objects != nil && object.storageKey != "" {
		data, err := p.objects.GetObject(ctx, object.storageBucket, object.storageKey)
		if err != nil {
			return "", fmt.Errorf("vkdelivery: load artifact for upload: %w", err)
		}
		filename := artifactFilename(artifact, filenamePrompt)
		switch artifact.MediaType {
		case domain.MediaTypeImage:
			return p.uploader.UploadPhoto(ctx, peerID, filename, data, object.mimeType)
		case domain.MediaTypeVideo:
			return p.uploader.UploadVideo(ctx, peerID, filename, data, object.mimeType)
		}
	}
	if p.signedURLs && p.signer != nil && object.storageKey != "" {
		signedURL, err := p.signer.PresignedGetURL(ctx, object.storageBucket, object.storageKey, p.urlTTL)
		if err != nil {
			return "", fmt.Errorf("vkdelivery: sign media delivery URL: %w", err)
		}
		if signedURL == "" {
			return "", errors.New("vkdelivery: signed media delivery URL is empty")
		}
		return signedURL, nil
	}
	return "", errors.New("vkdelivery: media delivery requires VK attachment or signed URL")
}

type mediaDeliveryObject struct {
	storageBucket string
	storageKey    string
	mimeType      string
}

func (p *Publisher) mediaObjectForDelivery(ctx context.Context, artifact *domain.Artifact) (mediaDeliveryObject, error) {
	object := mediaDeliveryObject{
		storageBucket: artifact.StorageBucket,
		storageKey:    artifact.StorageKey,
		mimeType:      artifact.MimeType,
	}
	if artifact.MediaType != domain.MediaTypeVideo {
		return object, nil
	}
	variants, err := p.artifacts.ListVariants(ctx, artifact.ID)
	if err != nil {
		return object, fmt.Errorf("vkdelivery: list artifact variants: %w", err)
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
			}, nil
		}
	}
	if readyOriginalVideo(artifact, p.rawVideoPolicy) {
		return object, nil
	}
	return object, errors.New("vkdelivery: video original is not allowed without ready variant")
}

func readyOriginalVideo(artifact *domain.Artifact, policy string) bool {
	if artifact == nil ||
		artifact.MediaType != domain.MediaTypeVideo ||
		artifact.StorageBucket == "" ||
		artifact.StorageKey == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "always_dev_only":
		return true
	case "if_probe_passed":
		return artifact.ProbeStatus == domain.MediaProbePassed &&
			strings.EqualFold(artifact.Container, "mp4") &&
			strings.EqualFold(artifact.Codec, "h264")
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

func attachmentRef(artifact *domain.Artifact) string {
	if artifact == nil {
		return ""
	}
	return artifact.PublicURL
}

func isVKAttachment(reference string) bool {
	return strings.HasPrefix(reference, "photo") ||
		strings.HasPrefix(reference, "video") ||
		strings.HasPrefix(reference, "doc")
}

func imageResultKeyboard() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]KeyboardButton{
			{deliveryButton("🔁 Сгенерировать ещё", domain.CommandMenuImageBackToQuality, "primary")},
			{deliveryButton("🏠 Главное меню", domain.CommandShowMenu, "secondary")},
		},
	}
}

func deliveryButton(label string, command domain.CommandType, color string) KeyboardButton {
	payload, _ := json.Marshal(struct {
		Command string `json:"command"`
	}{Command: string(command)})
	return KeyboardButton{
		Label:      label,
		Payload:    string(payload),
		Color:      color,
		ActionType: "text",
	}
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
		count := vkTextChunkLimit
		if len(runes) < count {
			count = len(runes)
		}
		cut := count
		for index := count - 1; index > 0; index-- {
			switch runes[index] {
			case '\n', ' ', '\t':
				cut = index + 1
				index = 0
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
	for index, line := range lines {
		lines[index] = formatVKLine(line)
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

func safeVKMediaFailureNotice(errorCode string) string {
	switch errorCode {
	case domain.JobErrMediaProviderOutputInvalid:
		return "Не удалось безопасно подготовить медиафайл. ⭐️ не списаны. Попробуйте изменить описание или повторить позже."
	case domain.JobErrMediaOverloadedRetryLater:
		return "Сейчас высокая нагрузка на генерацию медиа. ⭐️ не списаны. Попробуйте позже."
	case domain.JobErrMediaDeliveryFailed:
		return "Не удалось доставить готовый медиафайл. ⭐️ не списаны. Попробуйте позже."
	case domain.JobErrMediaProcessingUnavailable:
		return "Не удалось получить или подготовить готовый медиафайл. ⭐️ не списаны. Попробуйте позже."
	case domain.JobErrModelUnavailable, string(domain.ProviderErrModelUnavailable):
		return "Выбранная модель сейчас недоступна. ⭐️ не списаны. Попробуйте другую модель."
	case string(domain.ProviderErrRateLimited),
		string(domain.ProviderErrTimeout),
		string(domain.ProviderErrOverloaded),
		string(domain.ProviderErrInternal):
		return "Генерация временно недоступна. ⭐️ не списаны. Попробуйте позже."
	case string(domain.ProviderErrAuthFailed), string(domain.ProviderErrInsufficientBalance):
		return "Провайдер генерации временно недоступен. ⭐️ не списаны. Попробуйте позже."
	case string(domain.ProviderErrInvalidRequest):
		return "Модель не приняла запрос. ⭐️ не списаны. Попробуйте другую модель или измените описание; возможны ограничения по содержанию."
	case string(domain.ProviderErrUnsupportedCapab):
		return "Этот запрос не поддерживается выбранной моделью. ⭐️ не списаны. Измените параметры и попробуйте снова."
	case string(domain.ProviderErrContentRejected):
		return "Запрос отклонен правилами безопасности. ⭐️ не списаны. Измените описание и попробуйте снова."
	default:
		return "Генерация временно недоступна. ⭐️ не списаны. Попробуйте позже."
	}
}

func deliveryKind(deliveryType domain.DeliveryType) string {
	switch deliveryType {
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

type publisherPromptParams struct {
	Prompt                 string `json:"prompt"`
	VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
}

func promptFromJob(job *domain.Job) string {
	if job == nil || len(job.Params) == 0 {
		return ""
	}
	var params publisherPromptParams
	if err := json.Unmarshal(job.Params, &params); err != nil {
		return ""
	}
	return params.Prompt
}

func artifactFilename(artifact *domain.Artifact, prompt string) string {
	extension := "bin"
	switch artifact.MediaType {
	case domain.MediaTypeImage:
		extension = "png"
	case domain.MediaTypeVideo:
		extension = "mp4"
	}
	if base := promptFilenameBase(prompt, 25); base != "" {
		return base + "." + extension
	}
	return artifact.ID.String() + "." + extension
}

func promptFilenameBase(prompt string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	normalized := strings.Join(strings.Fields(prompt), " ")
	if normalized == "" {
		return ""
	}
	output := make([]rune, 0, maxRunes)
	for _, character := range normalized {
		if len(output) >= maxRunes {
			break
		}
		if unicode.IsControl(character) {
			continue
		}
		switch character {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			continue
		default:
			output = append(output, character)
		}
	}
	return strings.Trim(strings.TrimSpace(string(output)), ".")
}
