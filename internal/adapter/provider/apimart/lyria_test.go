package apimart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestLyriaNativeSubmissionAndPolling(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			calls++
			if r.URL.Path != "/v1/music/generations" {
				t.Errorf("path = %s", r.URL.Path)
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "flowmusic" || body["version"] != "lyria-3.5" || body["sound_prompt"] != "piano" || body["bpm"] != "120" || body["length"] != float64(60) || body["lyrics"] != "synthetic verse" || len(body) != 6 {
				t.Errorf("incorrect native fields: %v", body)
			}
			fmt.Fprint(w, `{"code":200,"data":[{"task_id":"flow-one","status":"submitted"}]}`)
			return
		}
		if r.URL.Path != "/v1/music/tasks/flow-one" || r.URL.Query().Get("language") != "en" {
			t.Errorf("wrong poll route")
		}
		fmt.Fprint(w, `{"code":200,"data":{"status":"completed","result":{"music":[{"clip_id":"clip-one","title":"Synthetic song","duration_seconds":"60.25","audio_url":"https://example.invalid/result.m4a","wav_url":"https://example.invalid/result.wav"}]}}}`)
	}))
	defer server.Close()
	p := New(Config{BaseURL: server.URL + "/v1", APIKey: "test", TaskLanguage: "ru"})
	req := domain.ProviderRequest{ModelCode: ModelLyria35, Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio, IdempotencyKey: "lyria-test", Music: &domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "piano", Lyrics: "synthetic verse", DurationSec: 60, BPM: 120}}
	task, err := p.submitLyria(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.submitLyria(context.Background(), req); err != nil || calls != 1 {
		t.Fatalf("duplicate submit %d, %v", calls, err)
	}
	result, err := p.pollLyria(context.Background(), domain.ProviderTaskRef{ExternalID: task.ExternalID})
	if err != nil || result.Status != domain.ProviderTaskSucceeded || result.Music == nil || len(result.Music.Tracks) != 1 {
		t.Fatalf("result failed: %v", err)
	}
	track := result.Music.Tracks[0]
	if track.OriginalAudioIndex != 1 || track.DurationSec != 60.25 || len(result.Music.Artifacts) != 1 {
		t.Fatal("track fields lost")
	}
}

func TestLyriaRejectsSunoControlsAndInvalidDuration(t *testing.T) {
	for _, change := range []func(*domain.MusicRequest){
		func(m *domain.MusicRequest) { m.Action = domain.MusicActionCover },
		func(m *domain.MusicRequest) { yes := true; m.MaxMode = &yes },
		func(m *domain.MusicRequest) { m.DurationSec = 241 },
		func(m *domain.MusicRequest) { m.DurationSec = -1 },
		func(m *domain.MusicRequest) { m.AudioURL = "https://example.invalid/audio.wav" },
	} {
		m := domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "piano"}
		change(&m)
		if domain.ValidateLyriaMusicRequest(m) == nil {
			t.Fatal("unsupported Lyria fields accepted")
		}
	}
}
