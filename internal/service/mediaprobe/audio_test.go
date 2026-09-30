package mediaprobe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type audioRunner struct {
	out    string
	args   []string
	stdin  []byte
	called bool
}

func (r *audioRunner) Run(_ context.Context, _ string, args []string, stdin []byte) ([]byte, error) {
	r.called = true
	r.args = append([]string(nil), args...)
	r.stdin = append([]byte(nil), stdin...)
	return []byte(r.out), nil
}

func TestFFProbeAudioAcceptsMP3ViaPipeOnly(t *testing.T) {
	runner := &audioRunner{out: validAudioProbeJSON("mp3", "mp3", "audio", "12.500000")}
	prober := NewFFProbe(Config{FFProbePath: "ffprobe", Timeout: time.Second}, WithRunner(runner))

	metadata, err := prober.ProbeAudio(context.Background(), []byte("mp3 bytes"), 9)
	if err != nil {
		t.Fatalf("ProbeAudio: %v", err)
	}
	if metadata.ProbeStatus != domain.MediaProbePassed || metadata.Container != "mp3" || metadata.Codec != "mp3" || metadata.DurationMS != 12500 {
		t.Fatalf("metadata = %+v", metadata)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "-protocol_whitelist pipe") || !strings.Contains(joined, "-i pipe:0") {
		t.Fatalf("ffprobe args did not force stdin pipe only: %#v", runner.args)
	}
	if string(runner.stdin) != "mp3 bytes" {
		t.Fatalf("stdin = %q", runner.stdin)
	}
}

func TestFFProbeAudioRejectsVideoStreamsAndExternalFormats(t *testing.T) {
	cases := map[string]string{
		"video_stream": `{"streams":[{"codec_type":"audio","codec_name":"aac","duration":"10"},{"codec_type":"video","codec_name":"h264","duration":"10"}],"format":{"format_name":"aac","duration":"10"}}`,
		"hls_format":   validAudioProbeJSON("hls,applehttp", "aac", "audio", "10"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			prober := NewFFProbe(Config{FFProbePath: "ffprobe", Timeout: time.Second}, WithRunner(&audioRunner{out: raw}))
			metadata, err := prober.ProbeAudio(context.Background(), []byte("audio"), 5)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if metadata.ProbeStatus != domain.MediaProbeFailed {
				t.Fatalf("ProbeStatus = %q", metadata.ProbeStatus)
			}
		})
	}
}

func TestFFProbeAudioRejectsOversizeAndInvalidDuration(t *testing.T) {
	runner := &audioRunner{out: validAudioProbeJSON("mp3", "mp3", "audio", "10")}
	prober := NewFFProbe(Config{FFProbePath: "ffprobe", Timeout: time.Second}, WithRunner(runner))
	if _, err := prober.ProbeAudio(context.Background(), []byte("x"), MaxMusicInputBytes+1); err == nil || !strings.Contains(err.Error(), "audio_size_exceeded") {
		t.Fatalf("expected size rejection, got %v", err)
	}
	if runner.called {
		t.Fatal("runner called for oversize audio")
	}
	payloadRunner := &audioRunner{out: validAudioProbeJSON("mp3", "mp3", "audio", "10")}
	payloadProber := NewFFProbe(Config{FFProbePath: "ffprobe", Timeout: time.Second}, WithRunner(payloadRunner))
	if _, err := payloadProber.ProbeAudio(context.Background(), make([]byte, MaxMusicInputBytes+1), 1); err == nil || !strings.Contains(err.Error(), "audio_size_exceeded") {
		t.Fatalf("expected payload size rejection, got %v", err)
	}
	if payloadRunner.called {
		t.Fatal("runner called for oversize audio payload")
	}

	for _, duration := range []string{"0", "481", "NaN", "+Inf"} {
		t.Run(duration, func(t *testing.T) {
			prober := NewFFProbe(Config{FFProbePath: "ffprobe", Timeout: time.Second}, WithRunner(&audioRunner{out: validAudioProbeJSON("mp3", "mp3", "audio", duration)}))
			if _, err := prober.ProbeAudio(context.Background(), []byte("audio"), 5); err == nil {
				t.Fatal("expected duration rejection")
			}
		})
	}
}

func TestSniffMusicInputMIMERecognizesADTSBeforeMP3Frame(t *testing.T) {
	mimeType, ok := SniffMusicInputMIME([]byte{0xff, 0xf1, 0x50, 0x80})
	if !ok || mimeType != "audio/aac" {
		t.Fatalf("SniffMusicInputMIME = %q, %v; want audio/aac, true", mimeType, ok)
	}
}

func TestValidateMusicInputArtifactEnforcesOwnerAndAudioPolicy(t *testing.T) {
	owner := uuid.New()
	artifact := validMusicArtifact(owner)
	if err := ValidateMusicInputArtifact(artifact, owner); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}

	cases := map[string]func(*domain.Artifact){
		"foreign_owner": func(a *domain.Artifact) { a.OwnerAccountID = uuid.New() },
		"not_ready":     func(a *domain.Artifact) { a.Status = domain.ArtifactStatusStored },
		"not_input":     func(a *domain.Artifact) { a.Kind = domain.ArtifactKindOutput },
		"not_audio":     func(a *domain.Artifact) { a.MediaType = domain.MediaTypeVideo },
		"bad_mime":      func(a *domain.Artifact) { a.MimeType = "audio/mp4" },
		"bad_codec":     func(a *domain.Artifact) { a.Codec = "opus" },
		"bad_container": func(a *domain.Artifact) { a.Container = "mp4" },
		"too_large":     func(a *domain.Artifact) { a.SizeBytes = MaxMusicInputBytes + 1 },
		"too_long":      func(a *domain.Artifact) { a.DurationMS = int64(MaxMusicInputDurationSec+1) * 1000 },
		"no_storage":    func(a *domain.Artifact) { a.StorageKey = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := *artifact
			mutate(&candidate)
			if err := ValidateMusicInputArtifact(&candidate, owner); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func validAudioProbeJSON(container, codec, streamType, duration string) string {
	return `{"streams":[{"codec_type":"` + streamType + `","codec_name":"` + codec + `","duration":"` + duration + `","bit_rate":"128000"}],"format":{"format_name":"` + container + `","duration":"` + duration + `","bit_rate":"128000"}}`
}

func validMusicArtifact(owner uuid.UUID) *domain.Artifact {
	return &domain.Artifact{
		ID:             uuid.New(),
		OwnerAccountID: owner,
		Kind:           domain.ArtifactKindInput,
		MediaType:      domain.MediaTypeAudio,
		MimeType:       "audio/mpeg",
		StorageBucket:  "artifacts",
		StorageKey:     "inputs/audio.mp3",
		SizeBytes:      1024,
		DurationMS:     12500,
		Codec:          "mp3",
		Container:      "mp3",
		BitrateBPS:     128000,
		ProbeStatus:    domain.MediaProbePassed,
		Status:         domain.ArtifactStatusReady,
	}
}
