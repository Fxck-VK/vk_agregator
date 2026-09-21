package websession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
)

type conversationGenerationRequest struct {
	ReferenceArtifactIDs []uuid.UUID `json:"reference_artifact_ids,omitempty"`
	Prompt               string      `json:"prompt"`
	ModelID              string      `json:"model_id,omitempty"`
	ImageQuality         string      `json:"image_quality,omitempty"`
	AspectRatio          string      `json:"aspect_ratio,omitempty"`
	OutputCount          int         `json:"output_count,omitempty"`
	Resolution           string      `json:"resolution,omitempty"`
	DurationSec          int         `json:"duration_sec,omitempty"`
}

type safeVideoModel struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	AllowedResolutions  []string         `json:"allowed_resolutions"`
	AllowedDurations    []int            `json:"allowed_durations_sec"`
	AllowedAspectRatios []string         `json:"allowed_aspect_ratios"`
	DefaultResolution   string           `json:"default_resolution"`
	DefaultDuration     int              `json:"default_duration_sec"`
	DefaultAspectRatio  string           `json:"default_aspect_ratio"`
	PriceByOption       map[string]int64 `json:"price_by_option"`
}

// Only ready text-to-video routes are selectable until the web supports owned
// reference uploads. Public product aliases and server prices are the contract.
func (h *Handler) conversationVideoRoutes() []productcatalog.VideoRoute {
	routes := make([]productcatalog.VideoRoute, 0)
	if h.deps.ImagePricing == nil {
		return routes
	}
	for _, route := range h.cfg.VideoRoutes {
		if !route.Enabled || route.RequiresStartImage || route.RequiresReferenceVideo || route.AutomaticDuration || len(route.AllowedReferenceImageCounts) > 0 && !slices.Contains(route.AllowedReferenceImageCounts, 0) {
			continue
		}
		key := pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(route.Alias), Resolution: route.DefaultResolution, DurationSec: route.DefaultDurationSec}
		if _, err := h.deps.ImagePricing.Snapshot(key); err == nil {
			routes = append(routes, route)
		}
	}
	return routes
}

func (h *Handler) listVideoModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	items := make([]safeVideoModel, 0)
	for _, route := range h.conversationVideoRoutes() {
		controls, ok := productcatalog.WorkspaceVideoControls(route, h.deps.ImagePricing)
		if !ok {
			continue
		}
		items = append(items, safeVideoModel{ID: route.Alias, Name: route.Name, Description: route.Description, AllowedResolutions: controls.AllowedResolutions, AllowedDurations: controls.AllowedDurationsSec, AllowedAspectRatios: controls.AllowedAspectRatios, DefaultResolution: controls.DefaultResolution, DefaultDuration: controls.DefaultDurationSec, DefaultAspectRatio: controls.DefaultAspectRatio, PriceByOption: controls.PriceByOption})
	}
	writeJSON(w, http.StatusOK, struct {
		Items []safeVideoModel `json:"items"`
	}{items})
}

func (h *Handler) isConversationMediaModel(id string) bool {
	for _, model := range h.cfg.ImageModels {
		if model.ID == id {
			return true
		}
	}
	for _, route := range h.conversationVideoRoutes() {
		if route.Alias == id {
			return true
		}
	}
	return false
}

func (h *Handler) resolveConversationMedia(req conversationGenerationRequest) (domain.OperationType, domain.Modality, map[string]any, pricingcatalog.PricingSnapshot, error) {
	for _, model := range h.cfg.ImageModels {
		if model.ID != req.ModelID {
			continue
		}
		if req.Resolution != "" || req.DurationSec != 0 {
			break
		}
		resolution, err := imagegeneration.NewResolver(h.cfg.ImageModels, h.deps.ImagePricing).Resolve(imagegeneration.Request{Prompt: req.Prompt, ModelID: req.ModelID, Quality: req.ImageQuality, AspectRatio: req.AspectRatio, OutputCount: req.OutputCount, ReferenceCount: len(req.ReferenceArtifactIDs)})
		if err != nil {
			return "", "", nil, pricingcatalog.PricingSnapshot{}, err
		}
		params := webImageJobParams{Prompt: req.Prompt, ModelID: resolution.Worker.ModelID, ModelName: resolution.Worker.ModelName, Provider: resolution.Worker.Provider, ModelCode: resolution.Worker.ModelCode, Size: resolution.Worker.Size, Resolution: resolution.Worker.Resolution, ImageQuality: resolution.Worker.ImageQuality, AspectRatio: resolution.Worker.AspectRatio, OutputCount: resolution.Worker.OutputCount}
		raw, _ := json.Marshal(params)
		values := make(map[string]any)
		_ = json.Unmarshal(raw, &values)
		if len(req.ReferenceArtifactIDs) > 0 {
			values["reference_artifact_ids"] = req.ReferenceArtifactIDs
		}
		return domain.OperationImageGenerate, domain.ModalityImage, values, resolution.PricingSnapshot, nil
	}
	for _, route := range h.conversationVideoRoutes() {
		if route.Alias != req.ModelID {
			continue
		}
		if len(req.ReferenceArtifactIDs) > 0 || req.ImageQuality != "" || req.OutputCount != 0 {
			break
		}
		if req.Resolution == "" {
			req.Resolution = route.DefaultResolution
		}
		if req.DurationSec == 0 {
			req.DurationSec = route.DefaultDurationSec
		}
		if req.AspectRatio == "" {
			req.AspectRatio = route.DefaultAspectRatio
		}
		if !slices.Contains(route.AllowedResolutions, req.Resolution) || !slices.Contains(route.AllowedDurationsSec, req.DurationSec) || !slices.Contains(route.AllowedAspectRatios, req.AspectRatio) {
			break
		}
		controls, available := productcatalog.WorkspaceVideoControls(route, h.deps.ImagePricing)
		validCombination := false
		if available {
			for _, variant := range controls.Variants {
				if variant.Resolution == req.Resolution && variant.DurationSec == req.DurationSec && variant.AspectRatio == req.AspectRatio {
					validCombination = true
					break
				}
			}
		}
		if !validCombination {
			break
		}
		snapshot, err := h.deps.ImagePricing.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(route.Alias), Resolution: req.Resolution, DurationSec: req.DurationSec})
		if err != nil {
			return "", "", nil, pricingcatalog.PricingSnapshot{}, err
		}
		return domain.OperationVideoGenerate, domain.ModalityVideo, map[string]any{"prompt": req.Prompt, "model_id": route.Alias, "model_name": route.Name, "video_route_alias": route.Alias, "resolution": req.Resolution, "duration_sec": req.DurationSec, "aspect_ratio": req.AspectRatio}, snapshot, nil
	}
	return "", "", nil, pricingcatalog.PricingSnapshot{}, errors.New("unavailable media model or options")
}

func (h *Handler) createConversationMedia(w http.ResponseWriter, r *http.Request, accountID uuid.UUID, conversation *domain.Conversation, key uuid.UUID, req conversationGenerationRequest) {
	operation, modality, params, snapshot, err := h.resolveConversationMedia(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media model options")
		return
	}
	if !h.validateConversationInputs(w, r, accountID, req) {
		return
	}
	params["conversation_id"] = conversation.ID.String()
	params["conversation_source"] = string(domain.ConversationSourceWeb)
	raw, _ := json.Marshal(params)
	orchestrationKey := "web-chat:" + accountID.String() + ":" + key.String()
	job, err := h.deps.WebChatJobs.CreateJob(r.Context(), joborchestrator.CreateJobInput{AccountID: accountID, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, ResultMode: domain.ResultModeAccountHistory, Operation: operation, Modality: modality, IdempotencyKey: orchestrationKey, CorrelationID: orchestrationKey, Params: raw, PricingSnapshot: snapshot})
	switch {
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "idempotency key belongs to another message")
		return
	case errors.Is(err, domain.ErrInsufficientCredits):
		writeError(w, http.StatusPaymentRequired, "insufficient credits")
		return
	case errors.Is(err, domain.ErrActiveJobLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "chat message rate limited")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "media generation unavailable")
		return
	}
	if !ownedConversationJob(job, accountID, conversation.ID) || job.OperationType != operation || job.Modality != modality || job.IdempotencyKey != orchestrationKey || job.CorrelationID != orchestrationKey {
		writeError(w, http.StatusServiceUnavailable, "media generation unavailable")
		return
	}
	var persisted map[string]any
	if json.Unmarshal(job.Params, &persisted) != nil {
		writeError(w, http.StatusServiceUnavailable, "media generation unavailable")
		return
	}
	// Workers may add their private resolved route; public intent must remain exact.
	var expected map[string]any
	_ = json.Unmarshal(raw, &expected)
	for key, value := range expected {
		if !reflect.DeepEqual(persisted[key], value) {
			writeError(w, http.StatusConflict, "idempotency key belongs to another message")
			return
		}
	}
	writePersistedWebChatJob(w, job)
}

func ownedConversationJob(job *domain.Job, accountID, conversationID uuid.UUID) bool {
	if job == nil || job.ID == uuid.Nil || job.AccountID != accountID || job.UserID != uuid.Nil || job.Source != "web" || job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil || job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" || job.VKPeerID != 0 || job.CommandID != uuid.Nil {
		return false
	}
	var params struct {
		ConversationID string `json:"conversation_id"`
		Source         string `json:"conversation_source"`
	}
	return json.Unmarshal(job.Params, &params) == nil && params.ConversationID == conversationID.String() && params.Source == string(domain.ConversationSourceWeb)
}

func (h *Handler) getConversationJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	conversationID, err := uuid.Parse(r.PathValue("conversationID"))
	jobID, jobErr := uuid.Parse(r.PathValue("jobID"))
	if err != nil || jobErr != nil || h.deps.ImageJobReader == nil || h.deps.Conversations == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	conv, err := h.deps.Conversations.GetByIDForAccount(r.Context(), principal.AccountID, conversationID)
	if err != nil || conv == nil || conv.Source != domain.ConversationSourceWeb || conv.Status != domain.ConversationActive {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), principal.AccountID, jobID)
	if err != nil || !ownedConversationJob(job, principal.AccountID, conversationID) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, safeWebChatJob{JobID: job.ID, Status: job.Status})
}

type safeConversationImage struct {
	Job      safeImageJob              `json:"job"`
	Artifact safeImageArtifactMetadata `json:"artifact"`
}
type safeConversationVideo struct {
	ID       uuid.UUID `json:"id"`
	MIMEType string    `json:"mime_type"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
}

// Attachments are derived from account-owned, succeeded, moderated job outputs,
// never from provider-written Markdown or client attachment metadata.
func (h *Handler) attachConversationMedia(ctx context.Context, owner, conversationID, jobID uuid.UUID, message *safeConversationMessage) bool {
	if h.deps.ImageJobReader == nil {
		return true
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(ctx, owner, jobID)
	if err != nil || job == nil || job.Modality == domain.ModalityText {
		return true
	}
	if !ownedConversationJob(job, owner, conversationID) || h.deps.ImageResults == nil {
		return false
	}
	result, err := h.deps.ImageResults.GetResult(ctx, owner, jobID)
	if err != nil || result.ID != job.ID || result.Status != domain.JobStatusSucceeded {
		return false
	}
	if job.Modality == domain.ModalityImage {
		safeJob, ok := newSafeImageJob(job)
		safeResult, resultOK := newSafeImageJobResult(result, job.ID)
		if !ok || !resultOK {
			return false
		}
		for _, artifact := range safeResult.Artifacts {
			message.Images = append(message.Images, safeConversationImage{Job: safeJob, Artifact: artifact})
		}
	} else if job.Modality == domain.ModalityVideo && result.Modality == domain.ModalityVideo && result.Operation == domain.OperationVideoGenerate {
		for _, artifact := range result.Artifacts {
			if artifact.ID == uuid.Nil || artifact.MediaType != domain.MediaTypeVideo || !strings.HasPrefix(artifact.MIMEType, "video/") {
				return false
			}
			message.Videos = append(message.Videos, safeConversationVideo{ID: artifact.ID, MIMEType: artifact.MIMEType, Width: artifact.Width, Height: artifact.Height})
		}
	}
	return true
}

func validArtifactJob(job *domain.Job, video bool) bool {
	if !video {
		_, ok := newSafeImageJob(job)
		return ok
	}
	if job == nil || job.OperationType != domain.OperationVideoGenerate || job.Modality != domain.ModalityVideo {
		return false
	}
	var params struct {
		ConversationID string `json:"conversation_id"`
	}
	if json.Unmarshal(job.Params, &params) != nil {
		return false
	}
	id, err := uuid.Parse(params.ConversationID)
	return err == nil && id != uuid.Nil && ownedConversationJob(job, job.AccountID, id)
}

func safeArtifactResultContains(result resultservice.Result, jobID, artifactID uuid.UUID, video bool) bool {
	if !video {
		safe, ok := newSafeImageJobResult(result, jobID)
		return ok && safeResultContainsArtifact(safe, artifactID)
	}
	if result.ID != jobID || result.Status != domain.JobStatusSucceeded || result.Operation != domain.OperationVideoGenerate || result.Modality != domain.ModalityVideo {
		return false
	}
	for _, artifact := range result.Artifacts {
		if artifact.ID == artifactID && artifact.MediaType == domain.MediaTypeVideo && artifact.SizeBytes > 0 && strings.HasPrefix(artifact.MIMEType, "video/") {
			return true
		}
	}
	return false
}
