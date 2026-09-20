package musicgeneration

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestMusicRequestsRejectNativeReferencesAndInvalidDimensions(t *testing.T) {
	base := Request{ModelID: "suno_v6", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Instrumental landscape"}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Request){
		func(r *Request) { r.Music.SourceTaskID = "native-id" }, func(r *Request) { r.Music.PersonaID = "native-id" }, func(r *Request) { r.Music.CustomModelID = "native-id" },
		func(r *Request) { r.Music.AudioURL = "https://example.test/private.mp3" }, func(r *Request) { r.Music.Action = "vox" },
		func(r *Request) { v := 1.1; r.Music.Weirdness = &v }, func(r *Request) { r.ModelID = "happyhorse_1_0" },
	} {
		r := base
		mutate(&r)
		if r.Validate() == nil {
			t.Errorf("unsafe intent accepted: %+v", r)
		}
	}
	zero := 0.0
	base.Music.Weirdness = &zero
	if base.Validate() != nil {
		t.Fatal("zero tuning weight rejected")
	}
	_, _, err := Resolve(base, providermodels.StaticRegistry())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unadmitted route resolved: %v", err)
	}
}

func TestMusicRequestValidateRejectsInvalidMaxCustomModeBeforeQuote(t *testing.T) {
	max := true
	request := Request{
		ModelID: "suno_v6",
		Music:   domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "idea", MaxMode: &max},
	}
	if err := request.Validate(); err == nil {
		t.Fatal("generate max without custom=true accepted")
	}

	customFalse := false
	request.Music.Custom = &customFalse
	if err := request.Validate(); err == nil {
		t.Fatal("generate max with custom=false accepted")
	}

	customTrue := true
	request.Music.Custom = &customTrue
	request.Music.Lyrics = "lyrics"
	if err := request.Validate(); err != nil {
		t.Fatalf("generate max with custom=true rejected: %v", err)
	}

	request = Request{
		ModelID: "suno_v6",
		Music:   domain.MusicRequest{Action: domain.MusicActionExtend, Prompt: "next verse", MaxMode: &max},
		Sources: []Source{{
			JobID:      uuid.New(),
			AudioIndex: 1,
		}},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("implicit-custom extend max rejected: %v", err)
	}
}

func TestEveryOperationHasExactPrices(t *testing.T) {
	seen := map[domain.MusicAction]bool{}
	for _, op := range Operations() {
		if seen[op.ID] || !op.ID.Valid() {
			t.Fatalf("duplicate/invalid %s", op.ID)
		}
		seen[op.ID] = true
		if _, err := pricingcatalog.MusicCandidateQuote("suno_v6", string(op.ID), false); err != nil {
			t.Errorf("missing price %s", op.ID)
		}
		if op.SupportsMax {
			if _, err := pricingcatalog.MusicCandidateQuote("suno_v6", string(op.ID), true); err != nil {
				t.Errorf("missing Max price %s", op.ID)
			}
		}
	}
	if len(seen) != 33 {
		t.Fatalf("lost operation: count %d", len(seen))
	}
}

func TestPersonaReferencesRespectActionAndCustomMode(t *testing.T) {
	customTrue, customFalse := true, false
	persona := uuid.New()
	for _, tc := range []struct {
		name   string
		action domain.MusicAction
		custom *bool
		source bool
		valid  bool
	}{
		{"generation needs custom mode", domain.MusicActionGenerate, nil, false, false},
		{"generation rejects description mode", domain.MusicActionGenerate, &customFalse, false, false},
		{"generation accepts custom lyrics", domain.MusicActionGenerate, &customTrue, false, true},
		{"extend defaults to custom", domain.MusicActionExtend, nil, true, true},
		{"extend rejects description mode", domain.MusicActionExtend, &customFalse, true, false},
		{"lyrics does not accept persona", domain.MusicActionLyrics, &customTrue, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Request{ModelID: "suno_v6", PersonaJobID: persona, Music: domain.MusicRequest{
				Action: tc.action, Custom: tc.custom, Prompt: "synthetic lyrics", GPTDescription: "synthetic idea",
			}}
			if tc.source {
				r.Sources = []Source{{JobID: uuid.New(), AudioIndex: 1}}
			}
			if err := r.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if tc.valid {
				r.CustomModelJobID = uuid.New()
				if r.Validate() == nil {
					t.Fatal("persona and custom model accepted together")
				}
			}
		})
	}
}
