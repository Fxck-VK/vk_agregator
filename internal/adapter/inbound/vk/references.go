package vk

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/artifactservice"
)

const (
	defaultVKArtifactBucket        = "artifacts"
	defaultVKReferenceMaxBytes     = 20 << 20
	defaultVKReferenceMaxDimension = 4096
	defaultVKReferenceMaxPixels    = 4096 * 4096
)

type vkPhotoReference struct {
	URL    string
	Width  int
	Height int
}

type vkReferenceArtifactMessages struct {
	MissingRequired   string
	Unsupported       string
	UploadUnavailable string
	DownloadFailed    string
	StoreFailed       string
}

type vkReferenceArtifactRequest struct {
	RequiresReferenceImage bool
	SupportsReferenceImage bool
	MaxReferenceImages     int
	AllowedAspectRatios    []string
	Messages               vkReferenceArtifactMessages
}

type vkReferenceArtifactResult struct {
	ArtifactIDs   []uuid.UUID
	AspectRatio   string
	Notice        string
	OK            bool
	HasReferences bool
}

func (h *Handler) prepareVideoReferenceArtifacts(ctx context.Context, userID uuid.UUID, spec videoModeSpec, attachments []vkAttachment) ([]uuid.UUID, string, string, bool) {
	result := h.prepareReferenceArtifacts(ctx, userID, vkReferenceArtifactRequest{
		RequiresReferenceImage: spec.RequiresStartImage,
		SupportsReferenceImage: spec.SupportsReferenceImage,
		MaxReferenceImages:     spec.MaxReferenceImages,
		AllowedAspectRatios:    spec.AllowedAspectRatios,
		Messages: vkReferenceArtifactMessages{
			MissingRequired:   "Для этой модели нужно прикрепить стартовое фото и написать описание одним сообщением.",
			Unsupported:       "Эта модель не принимает фото. Выберите другой режим видео.",
			UploadUnavailable: "Загрузка фото для видео сейчас недоступна. Попробуйте позже или выберите текстовую модель.",
			DownloadFailed:    "Не удалось загрузить фото из VK. Попробуйте отправить изображение заново.",
			StoreFailed:       "Не удалось сохранить фото для видео. Попробуйте позже.",
		},
	}, attachments)
	return result.ArtifactIDs, result.AspectRatio, result.Notice, result.OK
}

func (h *Handler) imageReferenceArtifactRequest(selection photoDialogSelection) vkReferenceArtifactRequest {
	supportsReferenceImage := selection.Model.SupportsReferenceImage
	maxReferenceImages := selection.Model.MaxReferenceImages
	if publicModel, ok := h.publicImageModel(selection.Model.ModelID); ok {
		supportsReferenceImage = publicModel.SupportsReferenceImage
		maxReferenceImages = publicModel.MaxReferenceImages
	}
	return vkReferenceArtifactRequest{
		SupportsReferenceImage: supportsReferenceImage,
		MaxReferenceImages:     maxReferenceImages,
		Messages: vkReferenceArtifactMessages{
			Unsupported:       "Эта модель не принимает фото. Отправьте текст без фото или выберите другую модель фото.",
			UploadUnavailable: "Загрузка фото для генерации сейчас недоступна. Попробуйте позже или отправьте текст без фото.",
			DownloadFailed:    "Не удалось загрузить фото из VK. Попробуйте отправить изображение заново.",
			StoreFailed:       "Не удалось сохранить фото для генерации. Попробуйте позже.",
		},
	}
}

func (h *Handler) prepareReferenceArtifacts(ctx context.Context, userID uuid.UUID, req vkReferenceArtifactRequest, attachments []vkAttachment) vkReferenceArtifactResult {
	photoRefs := vkPhotoReferences(attachments)
	result := vkReferenceArtifactResult{HasReferences: len(photoRefs) > 0}
	if len(photoRefs) == 0 {
		if req.RequiresReferenceImage {
			result.Notice = req.Messages.MissingRequired
			return result
		}
		result.OK = true
		return result
	}
	if !req.SupportsReferenceImage {
		result.Notice = req.Messages.Unsupported
		return result
	}
	maxRefs := req.MaxReferenceImages
	if maxRefs <= 0 {
		maxRefs = 1
	}
	if len(photoRefs) > maxRefs {
		result.Notice = fmt.Sprintf("Для этой модели можно прикрепить не больше %d фото.", maxRefs)
		return result
	}
	if h.cfg.ReferenceUploadsDisabled || h.deps.Artifacts == nil || h.deps.Objects == nil {
		result.Notice = req.Messages.UploadUnavailable
		return result
	}

	downloader := h.deps.Downloader
	if downloader == nil {
		downloader = artifactservice.NewHTTPDownloader()
	}
	saver := artifactservice.New(h.deps.Artifacts, h.deps.Objects, h.vkArtifactBucket())
	ids := make([]uuid.UUID, 0, len(photoRefs))
	aspectRatio := ""
	for _, ref := range photoRefs {
		data, _, err := downloader.Download(ctx, ref.URL)
		if err != nil {
			h.logger.Warn("vk reference photo download failed", "error_type", fmt.Sprintf("%T", err))
			result.Notice = req.Messages.DownloadFailed
			return result
		}
		mimeType, metadata, status, ok := h.validateVKReferenceImage(data)
		if !ok {
			result.Notice = status
			return result
		}
		if aspectRatio == "" && metadata.Width > 0 && metadata.Height > 0 {
			aspectRatio = closestVKAllowedAspectRatio(metadata.Width, metadata.Height, req.AllowedAspectRatios)
		}
		artifact, err := saver.SaveBytesArtifactWithMetadata(ctx, userID, nil, domain.ArtifactKindInput, domain.MediaTypeImage, mimeType, data, metadata)
		if err != nil {
			h.logger.Warn("vk reference photo store failed", "error_type", fmt.Sprintf("%T", err))
			result.Notice = req.Messages.StoreFailed
			return result
		}
		ids = append(ids, artifact.ID)
	}
	result.ArtifactIDs = ids
	result.AspectRatio = aspectRatio
	result.OK = true
	return result
}

func (h *Handler) vkArtifactBucket() string {
	if bucket := strings.TrimSpace(h.cfg.ArtifactBucket); bucket != "" {
		return bucket
	}
	return defaultVKArtifactBucket
}

func vkPhotoReferences(attachments []vkAttachment) []vkPhotoReference {
	refs := make([]vkPhotoReference, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment.Type != "photo" || attachment.Photo == nil {
			continue
		}
		if ref, ok := bestVKPhotoReference(attachment.Photo); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func bestVKPhotoReference(photo *vkPhoto) (vkPhotoReference, bool) {
	if photo == nil {
		return vkPhotoReference{}, false
	}
	var best vkPhotoReference
	bestPixels := -1
	for _, size := range photo.Sizes {
		url := strings.TrimSpace(size.URL)
		if url == "" {
			continue
		}
		pixels := size.Width * size.Height
		if pixels < 0 {
			pixels = 0
		}
		if best.URL == "" || pixels > bestPixels {
			best = vkPhotoReference{URL: url, Width: size.Width, Height: size.Height}
			bestPixels = pixels
		}
	}
	return best, best.URL != ""
}

func (h *Handler) validateVKReferenceImage(data []byte) (string, domain.ArtifactMediaMetadata, string, bool) {
	if len(data) == 0 {
		return "", domain.ArtifactMediaMetadata{}, "Фото не подходит. Загрузите JPG или PNG до 20 МБ.", false
	}
	maxBytes := h.cfg.MaxUploadBytes
	if maxBytes <= 0 {
		maxBytes = defaultVKReferenceMaxBytes
	}
	if int64(len(data)) > maxBytes {
		return "", domain.ArtifactMediaMetadata{}, "Фото слишком большое. Загрузите JPG или PNG до 20 МБ.", false
	}
	mimeType := http.DetectContentType(data)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		return "", domain.ArtifactMediaMetadata{}, "Фото не подходит. Загрузите JPG или PNG.", false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return "", domain.ArtifactMediaMetadata{}, "Фото не подходит. Загрузите другое JPG или PNG.", false
	}
	maxWidth := h.cfg.MaxUploadImageWidth
	if maxWidth <= 0 {
		maxWidth = defaultVKReferenceMaxDimension
	}
	maxHeight := h.cfg.MaxUploadImageHeight
	if maxHeight <= 0 {
		maxHeight = defaultVKReferenceMaxDimension
	}
	maxPixels := h.cfg.MaxUploadImagePixels
	if maxPixels <= 0 {
		maxPixels = defaultVKReferenceMaxPixels
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if cfg.Width > maxWidth || cfg.Height > maxHeight || pixels > maxPixels {
		return "", domain.ArtifactMediaMetadata{}, "Фото слишком большое. Загрузите изображение поменьше.", false
	}
	return mimeType, domain.ArtifactMediaMetadata{Width: cfg.Width, Height: cfg.Height, ProbeStatus: domain.MediaProbeSkipped}, "", true
}

func closestVKAllowedAspectRatio(width, height int, allowed []string) string {
	if width <= 0 || height <= 0 || len(allowed) == 0 {
		return ""
	}
	target := float64(width) / float64(height)
	orientation := vkAspectOrientation(width, height)
	if value := closestVKAspectRatio(target, orientation, allowed); value != "" {
		return value
	}
	return closestVKAspectRatio(target, 0, allowed)
}

func closestVKAspectRatio(target float64, orientation int, allowed []string) string {
	best := ""
	bestScore := math.MaxFloat64
	for _, raw := range allowed {
		value := strings.TrimSpace(raw)
		w, h, ok := parseVKAspectRatio(value)
		if !ok {
			continue
		}
		if orientation != 0 && vkAspectOrientation(w, h) != orientation {
			continue
		}
		score := math.Abs(math.Log(target / (float64(w) / float64(h))))
		if score < bestScore {
			best = value
			bestScore = score
		}
	}
	return best
}

func parseVKAspectRatio(value string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var width, height int
	if _, err := fmt.Sscanf(parts[0], "%d", &width); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &height); err != nil {
		return 0, 0, false
	}
	return width, height, width > 0 && height > 0
}

func vkAspectOrientation(width, height int) int {
	switch {
	case width > height:
		return 1
	case height > width:
		return -1
	default:
		return 0
	}
}

func (h *Handler) sendVideoReferenceNotice(ctx context.Context, idemKey string, peerID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		text = "Не удалось подготовить фото для видео. Попробуйте позже."
	}
	if h.deps.Control == nil {
		h.logger.Warn("vk video reference notice skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_video_reference:" + idemKey)
	_, err := h.sendControlMessage(ctx, domain.CommandMenuVideo, peerID, randomID, vkdelivery.Message{Text: text})
	return err
}

func (h *Handler) sendImageReferenceNotice(ctx context.Context, idemKey string, peerID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		text = "Не удалось подготовить фото для генерации. Попробуйте позже."
	}
	if h.deps.Control == nil {
		h.logger.Warn("vk image reference notice skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_image_reference:" + idemKey)
	_, err := h.sendControlMessage(ctx, domain.CommandMenuImage, peerID, randomID, vkdelivery.Message{Text: text})
	return err
}
