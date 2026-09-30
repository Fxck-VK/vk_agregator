package musicgeneration

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestLyriaRequestAndAdmission(t *testing.T) {
	r := Request{ModelID: "lyria_3_5", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "synthetic", DurationSec: 240}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Resolve(r, providermodels.StaticRegistry()); err != ErrUnavailable {
		t.Fatalf("unverified request: %v", err)
	}
	r.Music.DurationSec = 241
	if r.Validate() == nil {
		t.Fatal("accepted duration overflow")
	}
	r.Music.DurationSec = 1
	max := false
	r.Music.MaxMode = &max
	if r.Validate() == nil {
		t.Fatal("accepted Suno field")
	}
	if SameSourceFamily("suno_v6", "lyria_3_5") || SameSourceFamily("lyria_3_5", "suno_v6") {
		t.Fatal("cross API sources enabled")
	}
	if !SameSourceFamily("suno_v6", "suno_v6_mini") {
		t.Fatal("existing Suno sources disabled")
	}
	if ops := OperationsForModel("lyria_3_5"); len(ops) != 1 || ops[0].SupportsMax || ops[0].MaxSources != 0 {
		t.Fatal("wrong Lyria operations")
	}
	if len(OperationsForModel("whisper_1")) != 0 {
		t.Fatal("speech exposed as music")
	}
}
