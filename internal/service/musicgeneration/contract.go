// Package musicgeneration defines account-owned music intents. It calls no
// provider; native identifiers and fetch URLs are resolved only by workers.
package musicgeneration

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

var ErrInvalidRequest = errors.New("invalid music request")
var ErrUnavailable = errors.New("music operation unavailable")

// Source binds to an owned job and its original one-based provider track index.
type Source struct {
	JobID      uuid.UUID `json:"job_id"`
	AudioIndex int       `json:"audio_index"`
}

type Request struct {
	ModelID          string              `json:"model_id"`
	Music            domain.MusicRequest `json:"music"`
	Sources          []Source            `json:"sources,omitempty"`
	AudioArtifactIDs []uuid.UUID         `json:"audio_artifact_ids,omitempty"`
	PersonaJobID     uuid.UUID           `json:"persona_job_id,omitempty"`
	CustomModelJobID uuid.UUID           `json:"custom_model_job_id,omitempty"`
}

type JobParams struct {
	Request
	Provider  domain.ProviderName `json:"provider"`
	ModelCode string              `json:"model_code"`
	ModelName string              `json:"model_name"`
	// Result is a normalized private checkpoint, never sent as a public DTO.
	Result *StoredResult `json:"music_result,omitempty"`
}

type StoredResult struct {
	Music     domain.MusicResult `json:"music"`
	Artifacts []StoredArtifact   `json:"artifacts"`
	Complete  bool               `json:"complete"`
}
type StoredArtifact struct {
	ID         uuid.UUID `json:"id"`
	AudioIndex int       `json:"audio_index,omitempty"`
	Kind       string    `json:"kind"`
	Format     string    `json:"format,omitempty"`
}

func (r Request) Validate() error {
	m := r.Music
	if r.ModelID == "lyria_3_5" {
		if len(r.Sources) != 0 || len(r.AudioArtifactIDs) != 0 || r.PersonaJobID != uuid.Nil || r.CustomModelJobID != uuid.Nil || domain.ValidateLyriaMusicRequest(m) != nil {
			return ErrInvalidRequest
		}
		return nil
	}
	if _, ok := providermodels.MediaCandidateByID(r.ModelID); !ok || !m.Action.Valid() {
		return ErrInvalidRequest
	}
	if m.SourceTaskID != "" || len(m.SourceTaskIDs) > 0 || m.SourceAudioIndex != 0 || len(m.SourceAudioIndexes) > 0 || m.PersonaID != "" || m.CustomModelID != "" || m.AudioURL != "" || len(m.AudioURLs) > 0 {
		return ErrInvalidRequest
	}
	for _, v := range []*float64{m.StyleWeight, m.Weirdness, m.AudioWeight} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1) {
			return ErrInvalidRequest
		}
	}
	for _, v := range []*float64{m.StartSec, m.EndSec, m.ContinueAtSec, m.VocalStartSec, m.VocalEndSec, m.Speed} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
			return ErrInvalidRequest
		}
	}
	if len(r.Sources) > 2 || len(r.AudioArtifactIDs) > 24 {
		return ErrInvalidRequest
	}
	for i, s := range r.Sources {
		if s.JobID == uuid.Nil || s.AudioIndex < 1 || s.AudioIndex > 100 || slices.Contains(r.Sources[:i], s) {
			return ErrInvalidRequest
		}
	}
	for i, id := range r.AudioArtifactIDs {
		if id == uuid.Nil || slices.Contains(r.AudioArtifactIDs[:i], id) {
			return ErrInvalidRequest
		}
	}
	if r.PersonaJobID != uuid.Nil && r.CustomModelJobID != uuid.Nil {
		return ErrInvalidRequest
	}
	if utf8.RuneCountInString(m.Prompt) > 5000 || utf8.RuneCountInString(m.Lyrics) > 5000 || utf8.RuneCountInString(m.Title) > 80 || utf8.RuneCountInString(m.Style) > 1000 || len(m.GPTDescription) > 12000 {
		return ErrInvalidRequest
	}
	op, ok := OperationByID(m.Action)
	if !ok {
		return ErrInvalidRequest
	}
	if err := validateDocumentedSunoModes(m, op); err != nil {
		return err
	}
	if _, err := pricingcatalog.MusicCandidateQuote(r.ModelID, string(m.Action), m.MaxMode != nil && *m.MaxMode); err != nil {
		return ErrInvalidRequest
	}
	if len(r.Sources) < op.MinSources || len(r.Sources) > op.MaxSources || len(r.AudioArtifactIDs) < op.MinUploads || len(r.AudioArtifactIDs) > op.MaxUploads {
		return ErrInvalidRequest
	}
	if m.AudioFormat != "" && (!op.SupportsAudioFormat || !slices.Contains([]string{"mp3", "m4a", "wav"}, m.AudioFormat)) {
		return ErrInvalidRequest
	}
	if r.CustomModelJobID != uuid.Nil && !op.SupportsCustomModel {
		return ErrInvalidRequest
	}
	if r.PersonaJobID != uuid.Nil {
		if !op.SupportsPersona || boolPtrFalse(m.Custom) || (m.Action == domain.MusicActionGenerate && !boolPtrValue(m.Custom)) {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validateDocumentedSunoModes(m domain.MusicRequest, op Operation) error {
	if boolPtrValue(m.MaxMode) {
		if !op.SupportsMax {
			return ErrInvalidRequest
		}
		switch m.Action {
		case domain.MusicActionGenerate,
			domain.MusicActionUploadCover,
			domain.MusicActionCover,
			domain.MusicActionMashup,
			domain.MusicActionSample,
			domain.MusicActionAddVocals,
			domain.MusicActionAddInstrumental,
			domain.MusicActionAddStem:
			if !boolPtrValue(m.Custom) {
				return ErrInvalidRequest
			}
		case domain.MusicActionExtend:
			if boolPtrFalse(m.Custom) {
				return ErrInvalidRequest
			}
		}
	}
	switch m.Action {
	case domain.MusicActionGenerate:
		custom := boolPtrValue(m.Custom)
		instrumental := boolPtrValue(m.Instrumental)
		if (!custom || !instrumental) && musicPromptText(m) == "" {
			return ErrInvalidRequest
		}
	case domain.MusicActionUploadCover:
		if boolPtrFalse(m.Custom) && strings.TrimSpace(m.GPTDescription) == "" {
			return ErrInvalidRequest
		}
		if boolPtrValue(m.Custom) && !boolPtrValue(m.Instrumental) && musicPromptText(m) == "" {
			return ErrInvalidRequest
		}
	case domain.MusicActionCover,
		domain.MusicActionAddVocals,
		domain.MusicActionAddInstrumental,
		domain.MusicActionAddStem:
		if boolPtrFalse(m.Custom) && strings.TrimSpace(m.GPTDescription) == "" {
			return ErrInvalidRequest
		}
	case domain.MusicActionMashup, domain.MusicActionSample:
		if boolPtrFalse(m.Custom) && strings.TrimSpace(m.GPTDescription) == "" {
			return ErrInvalidRequest
		}
		if boolPtrValue(m.Custom) && !boolPtrValue(m.Instrumental) && musicPromptText(m) == "" {
			return ErrInvalidRequest
		}
	}
	return nil
}

func musicPromptText(m domain.MusicRequest) string {
	if prompt := strings.TrimSpace(m.Prompt); prompt != "" {
		return prompt
	}
	return strings.TrimSpace(m.Lyrics)
}

func boolPtrValue(v *bool) bool {
	return v != nil && *v
}

func boolPtrFalse(v *bool) bool {
	return v != nil && !*v
}

// Resolve is deliberately fail-closed for researched candidates until an exact
// verified operation is admitted through the canonical registry.
func Resolve(r Request, registry providermodels.Registry) (JobParams, pricingcatalog.PricingSnapshot, error) {
	if err := r.Validate(); err != nil {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, err
	}
	if !registry.MediaCandidateAdmitted(r.ModelID, string(r.Music.Action)) {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	c, _ := providermodels.MediaCandidateByID(r.ModelID)
	s, err := pricingcatalog.MusicCandidateQuote(r.ModelID, string(r.Music.Action), r.Music.MaxMode != nil && *r.Music.MaxMode)
	return JobParams{Request: r, Provider: c.Provider, ModelCode: c.ModelCode, ModelName: c.Name}, s, err
}

// DecodeJob protects against provider identifiers inserted into persisted user
// options and binds both price dimensions and actual worker operation.
func DecodeJob(job *domain.Job) (JobParams, error) {
	var p JobParams
	if job == nil || job.AccountID == uuid.Nil || job.OperationType != domain.OperationAudioMusic || job.Modality != domain.ModalityAudio || json.Unmarshal(job.Params, &p) != nil || p.Request.Validate() != nil {
		return p, ErrInvalidRequest
	}
	c, ok := providermodels.MediaCandidateByID(p.ModelID)
	if !ok || c.Provider != p.Provider || c.ModelCode != p.ModelCode {
		return p, ErrInvalidRequest
	}
	var price pricingcatalog.PricingSnapshot
	if json.Unmarshal(job.PricingSnapshot, &price) != nil || !price.Valid() || price.Key.AudioModelID != p.ModelID || price.Key.AudioAction != string(p.Music.Action) || price.Key.AudioMax != (p.Music.MaxMode != nil && *p.Music.MaxMode) || price.InternalCredits != job.CostEstimate {
		return p, ErrInvalidRequest
	}
	return p, nil
}
