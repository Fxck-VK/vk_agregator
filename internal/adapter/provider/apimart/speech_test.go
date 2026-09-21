package apimart

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestSpeechNativeTransport(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.URL.Path == "/v1/audio/speech" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 5 || body["model"] != "gpt-4o-mini-tts" || body["input"] != "synthetic" || body["response_format"] != "wav" {
				t.Errorf("wrong TTS shape")
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("RIFFxxxxWAVEfmt synthetic"))
			return
		}
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if h.Filename != "input.wav" || string(b) != "synthetic audio" || r.FormValue("model") != "whisper-1" || r.FormValue("response_format") != "json" || r.FormValue("language") != "ru" {
			t.Error("wrong multipart")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"synthetic transcription"}`)
	}))
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL + "/v1", APIKey: "test", HTTPClient: srv.Client()})
	tts := domain.ProviderRequest{ModelCode: ModelGPT4oMiniTTS, Operation: domain.OperationAudioTTS, Modality: domain.ModalityAudio, IdempotencyKey: "tts", Speech: &domain.SpeechRequest{Text: "synthetic", Voice: "alloy", Format: "wav", Speed: 1}}
	for i := 0; i < 1; i++ {
		task, err := p.submitSpeech(context.Background(), tts)
		if err != nil {
			t.Fatal(err)
		}
		if task.ImmediateResult == nil || task.ImmediateResult.InlineAudio == nil || task.Status != domain.ProviderTaskSucceeded {
			t.Fatal("missing inline audio")
		}
		raw, _ := json.Marshal(task)
		if strings.Contains(string(raw), "synthetic") {
			t.Fatal("content persisted")
		}
	}
	stt := domain.ProviderRequest{ModelCode: ModelWhisper1, Operation: domain.OperationAudioSTT, Modality: domain.ModalityText, IdempotencyKey: "stt", Speech: &domain.SpeechRequest{Format: "json", Language: "ru", FileExtension: "wav", FileBytes: []byte("synthetic audio")}}
	task, err := p.submitSpeech(context.Background(), stt)
	if err != nil {
		t.Fatal(err)
	}
	if task.ImmediateResult.Text != "synthetic transcription" {
		t.Fatal("wrong text")
	}
	if posts != 2 {
		t.Fatalf("posts %d", posts)
	}
}

func TestSpeechRejectsBeforeHTTPAndStopsUncertainReplay(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(502) }))
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL + "/v1", APIKey: "test", HTTPClient: srv.Client()})
	r := domain.ProviderRequest{ModelCode: ModelGPT4oMiniTTS, Operation: domain.OperationAudioTTS, Modality: domain.ModalityAudio, IdempotencyKey: "once", Speech: &domain.SpeechRequest{Text: strings.Repeat("я", 4097), Voice: "alloy", Format: "wav", Speed: 1}}
	if _, err := p.submitSpeech(context.Background(), r); err == nil {
		t.Fatal("accepted oversized input")
	}
	if posts != 0 {
		t.Fatal("invalid request charged")
	}
	r.Speech.Text = "synthetic"
	for i := 0; i < 2; i++ {
		_, err := p.submitSpeech(context.Background(), r)
		if e, ok := err.(*Error); !ok || e.Class != domain.ProviderErrSubmitIndeterminate {
			t.Fatalf("wrong error %v", err)
		}
	}
	if posts != 1 {
		t.Fatal("uncertain request replayed")
	}
}
