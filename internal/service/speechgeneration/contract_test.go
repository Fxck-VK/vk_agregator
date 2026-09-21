package speechgeneration

import (
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
		if _, _, err := Resolve(r, providermodels.StaticRegistry(), prices); err != ErrUnavailable {
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
