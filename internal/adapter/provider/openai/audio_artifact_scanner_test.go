package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestAudioArtifactScannerTranscribesAndModeratesMP3(t *testing.T) {
	const spokenText = "spoken words in the audio"
	data := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
	var moderationCalls int
	client := scanTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected method or missing test authorization for %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer r.MultipartForm.RemoveAll()
			if got := r.FormValue("model"); got != "gpt-transcribe" {
				t.Errorf("transcription model = %q", got)
			}
			f, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("open uploaded file: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer f.Close()
			if header.Filename != "audio.mp3" || header.Header.Get("Content-Type") != "audio/mpeg" {
				t.Errorf("uploaded file = %q, %q", header.Filename, header.Header.Get("Content-Type"))
			}
			if actual, err := io.ReadAll(f); err != nil || !bytes.Equal(actual, data) {
				t.Errorf("uploaded bytes did not match test MP3, err = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"text": spokenText})
		case "/v1/moderations":
			moderationCalls++
			var request moderationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Input != spokenText {
				t.Errorf("moderation did not receive the transcript, err = %v", err)
			}
			_, _ = io.WriteString(w, `{"results":[{"flagged":false}]}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	scanner := NewAudioArtifactScanner(NewModerator(ModerationConfig{APIKey: "test-key", HTTPClient: client}))
	if err := scanner.Scan(t.Context(), domain.MediaTypeAudio, "audio/mpeg", data); err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if moderationCalls != 1 {
		t.Fatalf("moderation called %d times, want once", moderationCalls)
	}
}

func TestAudioArtifactScannerHandlesUnsafeOrInvalidTranscription(t *testing.T) {
	const privateFixture = "private-fixture"
	for _, tc := range []struct {
		name          string
		status        int
		transcription string
		moderation    string
		wantClass     domain.ProviderErrorClass
		wantModerated bool
	}{
		{"instrumental", http.StatusOK, `{"text":""}`, "", "", false},
		{"missing transcript", http.StatusOK, `{"languages":[]}`, "", domain.ProviderErrInternal, false},
		{"oversized response", http.StatusOK, `{"text":"` + strings.Repeat("x", 1<<20) + `"}`, "", domain.ProviderErrInternal, false},
		{"provider rejection", http.StatusUnauthorized, privateFixture, "", domain.ProviderErrAuthFailed, false},
		{"flagged transcript", http.StatusOK, `{"text":"example"}`, `{"results":[{"flagged":true,"categories":{"` + privateFixture + `":true}}]}`, domain.ProviderErrContentRejected, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			moderated := false
			client := scanTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/audio/transcriptions":
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.transcription)
				case "/moderations":
					moderated = true
					_, _ = io.WriteString(w, tc.moderation)
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			scanner := NewAudioArtifactScanner(NewModerator(ModerationConfig{BaseURL: "https://example.test", HTTPClient: client}))
			err := scanner.Scan(t.Context(), domain.MediaTypeAudio, "audio/mpeg", []byte{0xff, 0xfb, 0x90, 0x64})
			if tc.wantClass == "" {
				if err != nil {
					t.Fatalf("Scan() = %v, want nil", err)
				}
			} else {
				var normalized *Error
				if !errors.As(err, &normalized) || normalized.Class != tc.wantClass {
					t.Fatalf("Scan() = %v, want class %s", err, tc.wantClass)
				}
				if strings.Contains(err.Error(), privateFixture) {
					t.Fatal("Scan returned private provider details")
				}
			}
			if moderated != tc.wantModerated {
				t.Fatalf("moderation called = %t, want %t", moderated, tc.wantModerated)
			}
		})
	}
}

func TestAudioArtifactScannerRejectsOtherPayloadFormatsBeforeUpload(t *testing.T) {
	scanner := NewAudioArtifactScanner(NewModerator(ModerationConfig{HTTPClient: &http.Client{
		Transport: scanRoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("invalid audio reached HTTP transport")
			return nil, errors.New("invalid audio")
		}),
	}}))
	err := scanner.Scan(t.Context(), domain.MediaTypeAudio, "audio/mpeg", []byte("not MPEG audio"))
	var normalized *Error
	if !errors.As(err, &normalized) || normalized.Class != domain.ProviderErrInvalidRequest {
		t.Fatalf("Scan() = %v, want invalid request", err)
	}
}

type scanRoundTripFunc func(*http.Request) (*http.Response, error)

func (f scanRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Run the test HTTP endpoint through the client's transport without an OS
// listener, so the request can be checked in a network-restricted sandbox.
func scanTestClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: scanRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, r)
			return recorder.Result(), nil
		}),
	}
}
