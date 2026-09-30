package apimart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestSunoModelNormalization(t *testing.T) {
	for _, model := range []string{ModelSunoV6, ModelSunoV6Wild, ModelSunoV6Mini} {
		if !isSunoModel(model) {
			t.Fatalf("%q should be a Suno model", model)
		}
	}
	for _, model := range []string{"suno", "suno-v5", ModelSeedance25, ""} {
		if isSunoModel(model) {
			t.Fatalf("%q should not be a Suno model", model)
		}
	}
	if got := sunoVersionFromModelCode(ModelSunoV6Wild); got != "v6-wild" {
		t.Fatalf("wild version = %q", got)
	}
}

func TestSubmitSunoGenerateMapsCustomLyricsAndZeroWeights(t *testing.T) {
	zero := 0.0
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/music/generations" {
			t.Fatalf("path = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "provider_submit:suno:generate" {
			t.Fatalf("idempotency header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_suno"}]}`))
	}))
	defer srv.Close()

	req := sunoProviderRequest(t, domain.MusicRequest{
		Action:       domain.MusicActionGenerate,
		Custom:       boolRef(true),
		Instrumental: boolRef(false),
		Lyrics:       "[Verse]\nNeon-lit streets",
		Title:        "Midnight Drive",
		Style:        "synthwave, female vocal",
		NegativeTags: "metal",
		AutoLyrics:   boolRef(false),
		VocalGender:  "female",
		StyleWeight:  &zero,
		Weirdness:    &zero,
		AudioWeight:  &zero,
		Variety:      "off",
		MaxMode:      boolRef(false),
		AudioFormat:  "wav",
		DurationSec:  120,
	})
	req.ModelCode = ModelSunoV6Wild
	req.IdempotencyKey = "provider_submit:suno:generate"

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	task, err := p.submitSuno(context.Background(), req)
	if err != nil {
		t.Fatalf("submit suno: %v", err)
	}
	if task.Provider != domain.ProviderAPIMart || task.ModelCode != ModelSunoV6Wild || task.ExternalID != "music:task_suno" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if string(task.Request) != "{}" {
		t.Fatalf("provider task request must be durable-empty, got %s", task.Request)
	}
	for key, want := range map[string]any{
		"model":                "suno",
		"version":              "v6-wild",
		"custom":               true,
		"instrumental":         false,
		"prompt":               "[Verse]\nNeon-lit streets",
		"title":                "Midnight Drive",
		"style":                "synthwave, female vocal",
		"negative_tags":        "metal",
		"auto_lyrics":          false,
		"vocal_gender":         "Female",
		"style_weight":         0.0,
		"weirdness_constraint": 0.0,
		"audio_weight":         0.0,
		"variety":              "off",
		"max_mode":             false,
		"audio_format":         "wav",
		"duration":             float64(120),
	} {
		if got := seen[key]; got != want {
			t.Fatalf("%s = %#v, want %#v; body=%#v", key, got, want, seen)
		}
	}
	if _, ok := seen["tags"]; ok {
		t.Fatalf("generate must use style, not tags: %#v", seen)
	}
}

func TestSubmitSunoCanonicalActionsUseDocumentedEndpoints(t *testing.T) {
	type actionCase struct {
		action       domain.MusicAction
		path         string
		allowVersion bool
		inputURLs    []string
	}
	cases := []actionCase{
		{domain.MusicActionGenerate, "/music/generations", true, nil},
		{domain.MusicActionLyrics, "/music/generations/lyrics", false, nil},
		{domain.MusicActionInspo, "/music/generations/inspo", true, []string{"https://cdn.example/inspo-1.mp3"}},
		{domain.MusicActionSounds, "/music/generations/sounds", true, nil},
		{domain.MusicActionUpsampleTags, "/music/generations/upsampleTags", false, nil},
		{domain.MusicActionUpload, "/music/generations/uploadTask", false, []string{"https://cdn.example/upload.mp3"}},
		{domain.MusicActionUploadCover, "/music/generations/uploadCover", true, []string{"https://cdn.example/cover-source.mp3"}},
		{domain.MusicActionUploadExtend, "/music/generations/uploadExtend", true, []string{"https://cdn.example/extend-source.mp3"}},
		{domain.MusicActionCreateModel, "/music/generations/createModel", false, sixAudioURLs()},
		{domain.MusicActionExtend, "/music/generations/extend", true, nil},
		{domain.MusicActionCover, "/music/generations/coverSong", true, nil},
		{domain.MusicActionRemaster, "/music/generations/remaster", false, nil},
		{domain.MusicActionStems, "/music/generations/stems", false, nil},
		{domain.MusicActionStemsAll, "/music/generations/stemsAll", false, nil},
		{domain.MusicActionAddVocals, "/music/generations/addVocals", true, nil},
		{domain.MusicActionAddInstrumental, "/music/generations/addInstrumental", true, nil},
		{domain.MusicActionAddStem, "/music/generations/addStem", true, nil},
		{domain.MusicActionVoice, "/music/generations/createVoice", false, []string{"https://cdn.example/voice.mp3"}},
		{domain.MusicActionPersona, "/music/generations/persona", false, nil},
		{domain.MusicActionReplaceSection, "/music/generations/replaceMusic", true, nil},
		{domain.MusicActionRemoveSection, "/music/generations/removeSection", false, nil},
		{domain.MusicActionCrop, "/music/generations/crop", false, nil},
		{domain.MusicActionFadeIn, "/music/generations/fadeIn", false, nil},
		{domain.MusicActionFadeOut, "/music/generations/fadeOut", false, nil},
		{domain.MusicActionAdjustSpeed, "/music/generations/adjustSpeed", false, nil},
		{domain.MusicActionConcat, "/music/generations/concat", false, nil},
		{domain.MusicActionMashup, "/music/generations/mashup", true, nil},
		{domain.MusicActionSample, "/music/generations/sample", true, nil},
		{domain.MusicActionMIDI, "/music/generations/midi", false, nil},
		{domain.MusicActionAlignedLyrics, "/music/generations/alignedLyrics", false, nil},
		{domain.MusicActionBPM, "/music/generations/bpm", false, nil},
		{domain.MusicActionGenerateVideo, "/music/generations/generateMp4", false, nil},
		{domain.MusicActionExport, "/music/generations/download", false, nil},
	}

	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			var seenPath string
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenPath = r.URL.Path
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_` + strings.ReplaceAll(string(tc.action), "_", "-") + `"}]}`))
			}))
			defer srv.Close()

			req := sunoProviderRequest(t, minimalSunoMusicRequest(tc.action))
			req.ModelCode = ModelSunoV6Mini
			req.IdempotencyKey = "provider_submit:suno:" + string(tc.action)
			req.InputURLs = tc.inputURLs
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			if _, err := p.submitSuno(context.Background(), req); err != nil {
				t.Fatalf("submit %s: %v", tc.action, err)
			}
			if seenPath != tc.path {
				t.Fatalf("path = %q, want %q", seenPath, tc.path)
			}
			if _, ok := body["model"]; tc.action != domain.MusicActionUpload && !ok {
				t.Fatalf("body omitted normalized model: %#v", body)
			}
			if got := body["version"]; tc.allowVersion {
				if got != "v6-mini" {
					t.Fatalf("version = %#v, want v6-mini; body=%#v", got, body)
				}
			} else if got != nil {
				t.Fatalf("version must be omitted for %s: %#v", tc.action, body)
			}
		})
	}
}

func TestSubmitSunoMashupMapsSourceTaskAndIndexArrays(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_mashup"}]}`))
	}))
	defer srv.Close()

	req := sunoProviderRequest(t, domain.MusicRequest{
		Action:             domain.MusicActionMashup,
		SourceTaskIDs:      []string{"music:task_left", "task_right"},
		SourceAudioIndexes: []int{3, 1},
		Custom:             boolRef(false),
		GPTDescription:     "blend both songs",
		Title:              "Blend",
		AudioFormat:        "mp3",
	})
	req.IdempotencyKey = "provider_submit:suno:mashup"
	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := p.submitSuno(context.Background(), req); err != nil {
		t.Fatalf("submit mashup: %v", err)
	}
	if got := stringSliceFromAny(seen["task_ids"]); !slices.Equal(got, []string{"task_left", "task_right"}) {
		t.Fatalf("task_ids = %#v", seen["task_ids"])
	}
	if got := intSliceFromAny(seen["audio_indexes"]); !slices.Equal(got, []int{3, 1}) {
		t.Fatalf("audio_indexes = %#v", seen["audio_indexes"])
	}
}

func TestSubmitSunoRejectsDocumentedCustomModeErrorsBeforeHTTP(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.MusicRequest)
	}{
		{
			name: "generation custom false still requires prompt even when instrumental",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionGenerate)
				r.Custom = boolRef(false)
				r.Instrumental = boolRef(true)
				r.Prompt = ""
				r.Lyrics = ""
			},
		},
		{
			name: "upload cover max requires explicit custom true",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionUploadCover)
				r.Custom = nil
				r.MaxMode = boolRef(true)
			},
		},
		{
			name: "cover custom false requires gpt description",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionCover)
				r.Custom = boolRef(false)
				r.GPTDescription = ""
			},
		},
		{
			name: "mashup custom false requires gpt description",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionMashup)
				r.Custom = boolRef(false)
				r.GPTDescription = ""
			},
		},
		{
			name: "sample custom true noninstrumental requires prompt",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionSample)
				r.Custom = boolRef(true)
				r.Instrumental = boolRef(false)
				r.Prompt = ""
				r.Lyrics = ""
			},
		},
		{
			name: "add vocals custom false requires gpt description",
			mutate: func(r *domain.MusicRequest) {
				*r = minimalSunoMusicRequest(domain.MusicActionAddVocals)
				r.Custom = boolRef(false)
				r.GPTDescription = ""
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				t.Fatalf("unexpected HTTP call for invalid request: %s", r.URL.Path)
			}))
			defer srv.Close()

			music := minimalSunoMusicRequest(domain.MusicActionGenerate)
			tc.mutate(&music)
			p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			if _, err := p.submitSuno(context.Background(), sunoProviderRequest(t, music)); err == nil {
				t.Fatal("submitSuno succeeded, want validation error")
			}
			if calls != 0 {
				t.Fatalf("HTTP calls = %d, want 0", calls)
			}
		})
	}
}

func TestSubmitSunoExtendMaxUsesImplicitCustomWithoutCustomField(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"task_extend"}]}`))
	}))
	defer srv.Close()

	music := minimalSunoMusicRequest(domain.MusicActionExtend)
	music.Custom = nil
	music.MaxMode = boolRef(true)
	p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	if _, err := p.submitSuno(context.Background(), sunoProviderRequest(t, music)); err != nil {
		t.Fatalf("submit extend: %v", err)
	}
	if _, ok := seen["custom"]; ok {
		t.Fatalf("extend body sent custom field: %#v", seen)
	}
	if seen["max_mode"] != true {
		t.Fatalf("max_mode = %#v, want true; body=%#v", seen["max_mode"], seen)
	}
}

func TestValidateSunoRequestRejectsBadInputs(t *testing.T) {
	valid := minimalSunoMusicRequest(domain.MusicActionGenerate)
	valid.StyleWeight = floatRef(0)
	valid.Weirdness = floatRef(0)
	valid.AudioWeight = floatRef(0)
	if err := validateSunoRequest(sunoProviderRequest(t, valid)); err != nil {
		t.Fatalf("zero weights should be valid: %v", err)
	}

	cases := []struct {
		name   string
		model  string
		mutate func(*domain.MusicRequest)
	}{
		{name: "unsupported model", model: ModelSeedance25, mutate: func(*domain.MusicRequest) {}},
		{name: "missing action", mutate: func(r *domain.MusicRequest) { r.Action = "" }},
		{name: "deprecated vox action", mutate: func(r *domain.MusicRequest) { r.Action = "vox" }},
		{name: "prompt too long", mutate: func(r *domain.MusicRequest) { r.Prompt = strings.Repeat("a", 3001) }},
		{name: "title too long", mutate: func(r *domain.MusicRequest) { r.Title = strings.Repeat("a", 81) }},
		{name: "style too long", mutate: func(r *domain.MusicRequest) { r.Style = strings.Repeat("a", 1001) }},
		{name: "weight above one", mutate: func(r *domain.MusicRequest) { r.StyleWeight = floatRef(1.01) }},
		{name: "mashup needs two sources", mutate: func(r *domain.MusicRequest) {
			*r = minimalSunoMusicRequest(domain.MusicActionMashup)
			r.SourceTaskIDs = []string{"one"}
		}},
		{name: "inspo allows four references", mutate: func(r *domain.MusicRequest) {
			*r = minimalSunoMusicRequest(domain.MusicActionInspo)
			r.AudioURLs = []string{"https://cdn.example/1.mp3", "https://cdn.example/2.mp3", "https://cdn.example/3.mp3", "https://cdn.example/4.mp3", "https://cdn.example/5.mp3"}
		}},
		{name: "audio index starts at one", mutate: func(r *domain.MusicRequest) {
			*r = minimalSunoMusicRequest(domain.MusicActionExtend)
			r.SourceAudioIndex = -1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			providerReq := sunoProviderRequest(t, req)
			if tc.model != "" {
				providerReq.ModelCode = tc.model
			}
			if err := validateSunoRequest(providerReq); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestPollSunoUsesMusicTaskEndpointAndNormalizesResults(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus domain.ProviderTaskStatus
		wantURLs   []string
		wantText   string
		check      func(*testing.T, domain.ProviderTaskResult)
	}{
		{
			name: "three music tracks",
			body: `{"code":200,"data":{"id":"task_done","status":"completed","progress":100,"result":{"music":[` +
				`{"audio_url":"https://private.example/a.mp3?token=1","title":"A"},` +
				`{"audio_url":"https://private.example/b.mp3?token=2","title":"B"},` +
				`{"audio_url":"https://private.example/c.mp3?token=3","title":"C"}` +
				`]}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
			wantURLs: []string{
				"https://private.example/a.mp3?token=1",
				"https://private.example/b.mp3?token=2",
				"https://private.example/c.mp3?token=3",
			},
		},
		{
			name:       "lyrics text result",
			body:       `{"code":200,"data":{"id":"task_lyrics","status":"completed","result":{"lyrics":[{"title":"Song","text":"line one\nline two","tags":"pop"}]}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
			wantText:   "line one\nline two",
		},
		{
			name:       "export files",
			body:       `{"code":200,"data":{"id":"task_export","status":"completed","result":{"files":[{"format":"wav","url":"https://private.example/song.wav?token=secret"},{"format":"mp3","url":"https://private.example/song.mp3?token=secret"}]}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
			wantURLs:   []string{"https://private.example/song.wav?token=secret", "https://private.example/song.mp3?token=secret"},
		},
		{
			name:       "persona id only",
			body:       `{"code":200,"data":{"id":"task_persona","status":"completed","result":{"persona_id":"persona_123","name":"Singer"}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
		},
		{
			name:       "upload audio id only",
			body:       `{"code":200,"data":{"id":"task_upload","status":"completed","result":{"audio_id":"audio_123","duration":91}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
			check: func(t *testing.T, res domain.ProviderTaskResult) {
				t.Helper()
				if res.Music == nil || len(res.Music.Tracks) != 1 {
					t.Fatalf("music tracks = %#v, want one upload receipt track", res.Music)
				}
				track := res.Music.Tracks[0]
				if track.OriginalAudioIndex != 1 || track.AudioID != "audio_123" || track.DurationSec != 91 {
					t.Fatalf("upload track = %#v, want original index 1 with audio id and duration", track)
				}
				if len(res.OutputURLs) != 0 {
					t.Fatalf("output urls = %#v, want none", res.OutputURLs)
				}
			},
		},
		{
			name:       "midi instruments are whitelisted",
			body:       `{"code":200,"data":{"id":"task_midi","status":"completed","result":{"state":"complete","instruments":[{"instrument":1,"note":60,"velocity":80,"url":"https://private.example/midi","audio_id":"native-secret","notes":[{"duration":1.5,"id":"native-nested"}]}]}}}`,
			wantStatus: domain.ProviderTaskSucceeded,
			check: func(t *testing.T, res domain.ProviderTaskResult) {
				t.Helper()
				if res.Music == nil || res.Music.MIDI == nil {
					t.Fatalf("midi result = %#v, want MIDI metadata", res.Music)
				}
				raw := string(res.Music.MIDI.Instruments)
				for _, want := range []string{"instrument", "note", "velocity", "duration"} {
					if !strings.Contains(raw, want) {
						t.Fatalf("midi instruments = %s, want key %q", raw, want)
					}
				}
				for _, forbidden := range []string{"private.example", "native-secret", "native-nested", "audio_id", "url"} {
					if strings.Contains(raw, forbidden) {
						t.Fatalf("midi instruments leaked %q: %s", forbidden, raw)
					}
				}
			},
		},
		{
			name:       "unknown remains processing",
			body:       `{"code":200,"data":{"id":"task_unknown","status":"unknown","progress":50}}`,
			wantStatus: domain.ProviderTaskProcessing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Path; got != "/music/tasks/task_done" {
					t.Fatalf("path = %q", got)
				}
				if got := r.URL.Query().Get("language"); got != "en" {
					t.Fatalf("language query = %q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			p := New(Config{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			res, err := p.pollSuno(context.Background(), domain.ProviderTaskRef{Provider: domain.ProviderAPIMart, ExternalID: "music:task_done"})
			if err != nil {
				t.Fatalf("poll suno: %v", err)
			}
			if res.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", res.Status, tt.wantStatus)
			}
			if !slices.Equal(res.OutputURLs, tt.wantURLs) {
				t.Fatalf("output urls = %#v, want %#v", res.OutputURLs, tt.wantURLs)
			}
			if res.Text != tt.wantText {
				t.Fatalf("text = %q, want %q", res.Text, tt.wantText)
			}
			if tt.check != nil {
				tt.check(t, res)
			}
			raw := string(res.Raw)
			for _, forbidden := range []string{"private.example", "token=secret", "line one"} {
				if strings.Contains(raw, forbidden) {
					t.Fatalf("raw metadata contains private material %q: %s", forbidden, raw)
				}
			}
		})
	}
}

func sunoProviderRequest(t *testing.T, music domain.MusicRequest) domain.ProviderRequest {
	t.Helper()
	raw, err := json.Marshal(music)
	if err != nil {
		t.Fatalf("marshal music request: %v", err)
	}
	return domain.ProviderRequest{
		JobID:          uuid.New(),
		UserID:         uuid.New(),
		Operation:      domain.OperationAudioMusic,
		Modality:       domain.ModalityAudio,
		ModelCode:      ModelSunoV6,
		Provider:       domain.ProviderAPIMart,
		Prompt:         music.Prompt,
		Music:          &music,
		Params:         raw,
		IdempotencyKey: "provider_submit:suno:test",
	}
}

func minimalSunoMusicRequest(action domain.MusicAction) domain.MusicRequest {
	start := 1.0
	end := 8.0
	continueAt := 12.0
	speed := 1.25
	req := domain.MusicRequest{
		Action:             action,
		Prompt:             "safe music idea",
		Lyrics:             "[Verse]\nSafe lyrics",
		GPTDescription:     "safe direction",
		Title:              "Safe Song",
		Style:              "pop",
		Tags:               "pop",
		SourceTaskID:       "music:source_task",
		SourceAudioIndex:   2,
		ContinueAtSec:      &continueAt,
		StartSec:           &start,
		EndSec:             &end,
		DurationSec:        30,
		Speed:              &speed,
		KeepPitch:          boolRef(true),
		Name:               "Safe Name",
		Description:        "Safe description",
		Styles:             "pop",
		StemType:           "lead_vocal",
		VariationCategory:  "vocal",
		InfillLyrics:       "replacement line",
		SoundType:          "one-shot",
		BPM:                120,
		Key:                "C",
		Formats:            []string{"wav"},
		Format:             "wav",
		AudioFormat:        "mp3",
		SourceTaskIDs:      []string{"source_a", "source_b"},
		SourceAudioIndexes: []int{1, 2},
	}
	if action == domain.MusicActionCreateModel {
		req.AudioURLs = sixAudioURLs()
	}
	if action == domain.MusicActionInspo {
		req.AudioURLs = []string{"https://cdn.example/inspo.mp3"}
	}
	if action == domain.MusicActionUpload || action == domain.MusicActionUploadCover || action == domain.MusicActionUploadExtend || action == domain.MusicActionVoice {
		req.AudioURL = "https://cdn.example/input.mp3"
	}
	return req
}

func sixAudioURLs() []string {
	return []string{
		"https://cdn.example/ref-1.mp3",
		"https://cdn.example/ref-2.mp3",
		"https://cdn.example/ref-3.mp3",
		"https://cdn.example/ref-4.mp3",
		"https://cdn.example/ref-5.mp3",
		"https://cdn.example/ref-6.mp3",
	}
}

func stringSliceFromAny(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprint(value))
	}
	return out
}

func intSliceFromAny(value any) []int {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(values))
	for _, value := range values {
		n, ok := value.(float64)
		if !ok {
			return nil
		}
		out = append(out, int(n))
	}
	return out
}

func boolRef(value bool) *bool {
	return &value
}

func floatRef(value float64) *float64 {
	return &value
}
