// Package speechgeneration defines speech jobs without provider IO. Quotes are
// server-owned; the fixed DEV retail policy does not infer provider audio tokens.
package speechgeneration

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

var ErrInvalidRequest = errors.New("invalid speech request")
var ErrUnavailable = errors.New("speech operation awaits admission and verified pricing")

type Request struct {
	ModelID         string               `json:"model_id"`
	Speech          domain.SpeechRequest `json:"speech"`
	AudioArtifactID uuid.UUID            `json:"audio_artifact_id,omitempty"`
}
type JobParams struct {
	Request
	InputDurationMS int64               `json:"input_duration_ms,omitempty"`
	Provider        domain.ProviderName `json:"provider"`
	ModelCode       string              `json:"model_code"`
}
type PriceCatalog interface {
	Snapshot(pricingcatalog.ProductKey) (pricingcatalog.PricingSnapshot, error)
}

func (r Request) Operation() domain.OperationType {
	if r.ModelID == "gpt_4o_mini_tts" {
		return domain.OperationAudioTTS
	}
	if r.ModelID == "whisper_1" {
		return domain.OperationAudioSTT
	}
	return ""
}
func (r Request) Modality() domain.Modality {
	if r.Operation() == domain.OperationAudioSTT {
		return domain.ModalityText
	}
	return domain.ModalityAudio
}
func (r Request) Action() string {
	if r.Operation() == domain.OperationAudioTTS {
		return "speak"
	}
	return "transcribe"
}
func (r Request) Key() pricingcatalog.ProductKey {
	return pricingcatalog.ProductKey{Operation: r.Operation(), Modality: r.Modality(), AudioModelID: r.ModelID, AudioAction: r.Action()}
}
func (r Request) Validate() error {
	if r.Operation() == "" || domain.ValidateSpeechRequest(r.Speech, r.Operation(), false) != nil || len(r.Speech.FileBytes) > 0 || r.Speech.FileExtension != "" {
		return ErrInvalidRequest
	}
	if (r.Operation() == domain.OperationAudioSTT) != (r.AudioArtifactID != uuid.Nil) {
		return ErrInvalidRequest
	}
	// WAV is the initially wired, inspected output. Other native formats stay
	// documented API capabilities until the artifact pipeline supports them.
	if r.Operation() == domain.OperationAudioTTS && r.Speech.Format != "wav" {
		return ErrInvalidRequest
	}
	if r.Operation() == domain.OperationAudioSTT && r.Speech.Format != "json" {
		return ErrInvalidRequest
	}
	return nil
}
func (p JobParams) Key() pricingcatalog.ProductKey {
	k := p.Request.Key()
	if p.Operation() == domain.OperationAudioSTT {
		seconds, err := pricingcatalog.WhisperBillableDurationSec(p.InputDurationMS)
		if err != nil {
			k.DurationSec = -1
		} else {
			k.DurationSec = seconds
		}
	}
	return k
}

func (p JobParams) Validate() error {
	if p.Request.Validate() != nil {
		return ErrInvalidRequest
	}
	if p.Operation() == domain.OperationAudioSTT {
		if _, err := pricingcatalog.WhisperBillableDurationSec(p.InputDurationMS); err != nil {
			return ErrInvalidRequest
		}
	} else if p.InputDurationMS != 0 {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateTranscriptionInput(a *domain.Artifact, owner uuid.UUID) error {
	if mediaprobe.ValidateMusicInputArtifact(a, owner) != nil || a.SizeBytes > domain.WhisperMaxInputBytes || (a.MimeType != "audio/mpeg" && a.MimeType != "audio/wav" && a.MimeType != "audio/x-wav") {
		return ErrInvalidRequest
	}
	if _, err := pricingcatalog.WhisperBillableDurationSec(a.DurationMS); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func Resolve(r Request, registry providermodels.Registry, prices PriceCatalog, inputDurationMS int64) (JobParams, pricingcatalog.PricingSnapshot, error) {
	if err := r.Validate(); err != nil {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, err
	}
	if !registry.MediaCandidateRunnable(r.ModelID, r.Action()) || prices == nil {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	c, ok := providermodels.MediaCandidateByID(r.ModelID)
	if !ok {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	p := JobParams{Request: r, Provider: c.Provider, ModelCode: c.ModelCode, InputDurationMS: inputDurationMS}
	if err := p.Validate(); err != nil {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, err
	}
	s, err := prices.Snapshot(p.Key())
	if err != nil || !s.Valid() || s.Key != p.Key() {
		return JobParams{}, pricingcatalog.PricingSnapshot{}, ErrUnavailable
	}
	return p, s, nil
}
func DecodeJob(job *domain.Job) (JobParams, error) {
	var p JobParams
	if job == nil || job.AccountID == uuid.Nil || json.Unmarshal(job.Params, &p) != nil || p.Validate() != nil || p.Operation() != job.OperationType || p.Modality() != job.Modality {
		return p, ErrInvalidRequest
	}
	c, ok := providermodels.MediaCandidateByID(p.ModelID)
	if !ok || c.Provider != p.Provider || c.ModelCode != p.ModelCode {
		return p, ErrInvalidRequest
	}
	var price pricingcatalog.PricingSnapshot
	if json.Unmarshal(job.PricingSnapshot, &price) != nil || !price.Valid() || price.Key != p.Key() || price.InternalCredits != job.CostEstimate {
		return p, ErrInvalidRequest
	}
	return p, nil
}
