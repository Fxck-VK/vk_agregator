package speechgeneration

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestSpeechRequestsFailClosedWithoutAdmissionAndAudioMetering(t *testing.T) {
	prices, _ := pricingcatalog.NewStaticCatalog()
	for _, r := range []Request{
		{ModelID: "gpt_4o_mini_tts", Speech: domain.SpeechRequest{Text: "synthetic", Voice: "alloy", Format: "wav", Speed: 1}},
		{ModelID: "whisper_1", AudioArtifactID: uuid.New(), Speech: domain.SpeechRequest{Format: "json", Language: "ru"}},
	} {
		if r.Validate() != nil || !r.Key().Valid() {
			t.Fatalf("invalid typed request %s", r.ModelID)
		}
		if _, _, err := Resolve(r, providermodels.StaticRegistry(), prices, 0); err != ErrUnavailable {
			t.Fatal("unverified speech enabled")
		}
		if _, err := prices.Snapshot(r.Key()); err == nil {
			t.Fatal("invented speech retail price")
		}
		r.Speech.FileBytes = []byte("client file")
		if r.Validate() == nil {
			t.Fatal("client bytes accepted")
		}
	}
	r := Request{ModelID: "whisper_1", Speech: domain.SpeechRequest{Format: "json"}}
	if r.Validate() == nil {
		t.Fatal("missing source accepted")
	}
	r.ModelID = "suno_v6"
	if r.Validate() == nil {
		t.Fatal("music sent through speech")
	}
}

func TestWhisperDEVQuoteBindsInspectedDuration(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	prices, _ := pricingcatalog.NewStaticCatalog()
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	r := Request{ModelID: "whisper_1", AudioArtifactID: uuid.New(), Speech: domain.SpeechRequest{Format: "json", Language: "ru"}}
	p, q, err := Resolve(r, providermodels.RuntimeRegistry(), prices, 60001)
	if err != nil || q.InternalCredits != 50 || p.InputDurationMS != 60001 || p.Key().DurationSec != 120 {
		t.Fatalf("server quote = %+v, %+v, %v", p, q, err)
	}
	raw, _ := json.Marshal(p)
	price, _ := json.Marshal(q)
	job := &domain.Job{AccountID: uuid.New(), OperationType: domain.OperationAudioSTT, Modality: domain.ModalityText, Params: raw, PricingSnapshot: price, CostEstimate: 50}
	if _, err := DecodeJob(job); err != nil {
		t.Fatal(err)
	}
	p.InputDurationMS = 60000
	job.Params, _ = json.Marshal(p)
	if _, err := DecodeJob(job); err == nil {
		t.Fatal("changed duration retained a different minute price")
	}
	for _, ms := range []int64{-1, 0, 480001} {
		if _, _, err := Resolve(r, providermodels.RuntimeRegistry(), prices, ms); err == nil {
			t.Fatalf("duration accepted: %d", ms)
		}
	}
	if _, _, err := Resolve(r, providermodels.StaticRegistry(), prices, 60001); err != ErrUnavailable {
		t.Fatal("Whisper leaked into production")
	}
}

func TestWhisperInputMustBeOwnedInspectedAndBounded(t *testing.T) {
	owner := uuid.New()
	a := domain.Artifact{OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeAudio, Status: domain.ArtifactStatusReady, StorageBucket: "private", StorageKey: "input", MimeType: "audio/wav", DurationMS: 60001, SizeBytes: 100, Codec: "pcm_s16le", Container: "wav", ProbeStatus: domain.MediaProbePassed}
	if ValidateTranscriptionInput(&a, owner) != nil {
		t.Fatal("valid input rejected")
	}
	for _, change := range []func(*domain.Artifact){
		func(a *domain.Artifact) { a.OwnerAccountID = uuid.New() },
		func(a *domain.Artifact) { a.DurationMS = 0 }, func(a *domain.Artifact) { a.DurationMS = 480001 },
		func(a *domain.Artifact) { a.SizeBytes = 25_000_001 }, func(a *domain.Artifact) { a.ProbeStatus = "pending" },
		func(a *domain.Artifact) { a.MimeType = "audio/aac"; a.Container = "aac"; a.Codec = "aac" },
	} {
		bad := a
		change(&bad)
		if ValidateTranscriptionInput(&bad, owner) == nil {
			t.Fatal("untrusted speech input accepted")
		}
	}
}
