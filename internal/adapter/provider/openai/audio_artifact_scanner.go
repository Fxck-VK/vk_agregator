package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	artifactTranscriptionModel   = "gpt-transcribe"
	maxArtifactAudioBytes        = 20 << 20
	maxArtifactTranscriptReply   = 256 << 10
	artifactTranscriptionTimeout = 90 * time.Second
)

// AudioArtifactScanner adds transcript moderation for MP3 artifacts.
type AudioArtifactScanner struct {
	moderator         *Moderator
	transcriptionHTTP *http.Client
}

// NewAudioArtifactScanner wraps the OpenAI moderator for MP3 downloads.
func NewAudioArtifactScanner(moderator *Moderator) *AudioArtifactScanner {
	client := moderator.http
	if moderator.cfg.HTTPClient == nil {
		// Uploading and transcribing long tracks can take longer than text
		// moderation. Reuse the transport with a separate bounded timeout.
		transcriptionClient := *client
		transcriptionClient.Timeout = artifactTranscriptionTimeout
		client = &transcriptionClient
	}
	return &AudioArtifactScanner{moderator: moderator, transcriptionHTTP: client}
}

// Scan inspects MP3 audio by moderating its transcript in memory. It delegates
// other artifact media to the existing moderator.
func (s *AudioArtifactScanner) Scan(ctx context.Context, mediaType domain.MediaType, mimeType string, data []byte) error {
	if mediaType != domain.MediaTypeAudio {
		return s.moderator.Scan(ctx, mediaType, mimeType, data)
	}
	contentType, _, err := mime.ParseMediaType(mimeType)
	if err != nil || contentType != "audio/mpeg" || len(data) > maxArtifactAudioBytes || !hasMP3Header(data) {
		return &Error{Class: domain.ProviderErrInvalidRequest, Message: "invalid MP3 artifact"}
	}

	transcript, err := s.transcribe(ctx, data)
	if err != nil {
		return err
	}
	// An explicit empty transcript has no speech for text moderation.
	if strings.TrimSpace(transcript) == "" {
		return nil
	}
	if err := s.moderator.Scan(ctx, domain.MediaTypeText, "text/plain", []byte(transcript)); err != nil {
		var upstream *Error
		if errors.As(err, &upstream) {
			return &Error{Class: upstream.Class, Message: "audio transcript moderation failed"}
		}
		return &Error{Class: domain.ProviderErrContentRejected, Message: "audio transcript rejected"}
	}
	return nil
}

func hasMP3Header(data []byte) bool {
	if len(data) >= 10 && bytes.HasPrefix(data, []byte("ID3")) {
		return true
	}
	// MP3 can start directly with an MPEG Layer III frame without an ID3 tag.
	return len(data) >= 4 && data[0] == 0xff && data[1]&0xe6 == 0xe2 && data[1]&0x18 != 0x08
}

func (s *AudioArtifactScanner) transcribe(ctx context.Context, data []byte) (string, error) {
	invalidResponse := &Error{Class: domain.ProviderErrInternal, Message: "invalid audio transcription response"}
	setupFailed := &Error{Class: domain.ProviderErrInternal, Message: "audio transcription setup failed"}

	// Assemble the small multipart envelope separately to avoid duplicating the
	// artifact in memory. All parts have known sizes.
	var envelope bytes.Buffer
	writer := multipart.NewWriter(&envelope)
	if err := writer.WriteField("model", artifactTranscriptionModel); err != nil {
		return "", setupFailed
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="audio.mp3"`)
	header.Set("Content-Type", "audio/mpeg")
	if _, err := writer.CreatePart(header); err != nil {
		return "", setupFailed
	}
	dataOffset := envelope.Len()
	if err := writer.Close(); err != nil {
		return "", setupFailed
	}

	body := io.MultiReader(
		bytes.NewReader(envelope.Bytes()[:dataOffset]),
		bytes.NewReader(data),
		bytes.NewReader(envelope.Bytes()[dataOffset:]),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.moderator.cfg.BaseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", setupFailed
	}
	req.ContentLength = int64(envelope.Len() + len(data))
	req.Header.Set("Authorization", "Bearer "+s.moderator.cfg.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.transcriptionHTTP.Do(req)
	if err != nil {
		return "", &Error{Class: domain.ProviderErrTimeout, Message: "audio transcription request failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		class := classifyStatus(resp.StatusCode)
		return "", &Error{Class: class, Message: safeHTTPErrorMessage("openai audio transcription", class, resp.StatusCode)}
	}

	reply, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactTranscriptReply+1))
	if err != nil || len(reply) > maxArtifactTranscriptReply {
		return "", invalidResponse
	}
	var decoded struct {
		Text  *string         `json:"text"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(reply, &decoded); err != nil || decoded.Text == nil || (len(decoded.Error) != 0 && string(decoded.Error) != "null") {
		return "", invalidResponse
	}
	return *decoded.Text, nil
}
