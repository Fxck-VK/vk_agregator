package mediaprobe

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	// MaxMusicInputBytes is the local application upload policy for private music
	// inputs. It is independent from provider-native limits, which remain unknown
	// until route admission.
	MaxMusicInputBytes int64 = 25 << 20
	// MaxMusicInputDurationSec is the local application upload policy for private
	// music inputs. It deliberately does not reuse video probe config.
	MaxMusicInputDurationSec = 480
)

// ProbeAudio validates private audio input bytes and returns safe metadata. The
// data is passed to ffprobe through stdin with a pipe-only protocol whitelist;
// URL and playlist/container fetch formats are rejected by the allowlist below.
func (p *FFProbe) ProbeAudio(ctx context.Context, data []byte, sizeBytes int64) (domain.ArtifactMediaMetadata, error) {
	actualSize := sizeBytes
	if actualSize <= 0 {
		actualSize = int64(len(data))
	}
	if actualSize <= 0 || len(data) == 0 {
		return failedMetadata(), ProbeError{Reason: "empty_audio"}
	}
	if actualSize > MaxMusicInputBytes || int64(len(data)) > MaxMusicInputBytes {
		return failedMetadata(), ProbeError{Reason: "audio_size_exceeded"}
	}
	if strings.TrimSpace(p.cfg.FFProbePath) == "" {
		return failedMetadata(), ProbeError{Reason: "ffprobe_not_configured"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	out, err := p.runner.Run(probeCtx, p.cfg.FFProbePath, []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_packets",
		"-show_entries", "packet=duration_time:stream=codec_type,codec_name,duration,bit_rate:format=format_name,duration,bit_rate",
		"-read_intervals", "%+481",
		"-protocol_whitelist", "pipe",
		"-i", "pipe:0",
	}, data)
	if err != nil {
		if probeCtx.Err() != nil {
			return failedMetadata(), ProbeError{Reason: "probe_timeout"}
		}
		return failedMetadata(), ProbeError{Reason: "probe_failed"}
	}

	metadata, err := parseAndValidateAudio(out)
	if err != nil {
		return metadata, err
	}
	metadata.ProbeStatus = domain.MediaProbePassed
	return metadata, nil
}

// SniffMusicInputMIME identifies the only browser-uploaded audio byte shapes the
// application currently accepts. It is a quick header check; ProbeAudio remains
// the authoritative content validation.
func SniffMusicInputMIME(data []byte) (string, bool) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return "audio/wav", true
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return "audio/mpeg", true
	}
	if len(data) >= 2 && data[0] == 0xff && data[1]&0xf6 == 0xf0 {
		return "audio/aac", true
	}
	if len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0 {
		return "audio/mpeg", true
	}
	return "", false
}

// ValidateMusicInputArtifact is the shared server-side authorization gate for
// persisted private music input artifacts before they can be signed for a
// provider or hydrated into a worker request.
func ValidateMusicInputArtifact(artifact *domain.Artifact, accountID uuid.UUID) error {
	if artifact == nil || accountID == uuid.Nil || artifact.OwnerAccountID != accountID {
		return ProbeError{Reason: "music_artifact_not_found"}
	}
	if artifact.Kind != domain.ArtifactKindInput {
		return ProbeError{Reason: "music_artifact_not_input"}
	}
	if artifact.MediaType != domain.MediaTypeAudio {
		return ProbeError{Reason: "music_artifact_not_audio"}
	}
	if artifact.Status != domain.ArtifactStatusReady {
		return ProbeError{Reason: "music_artifact_not_ready"}
	}
	if strings.TrimSpace(artifact.StorageBucket) == "" || strings.TrimSpace(artifact.StorageKey) == "" {
		return ProbeError{Reason: "music_artifact_storage_missing"}
	}
	if artifact.SizeBytes <= 0 || artifact.SizeBytes > MaxMusicInputBytes {
		return ProbeError{Reason: "music_artifact_size"}
	}
	metadata := domain.ArtifactMediaMetadata{
		DurationMS:  artifact.DurationMS,
		Codec:       artifact.Codec,
		Container:   artifact.Container,
		BitrateBPS:  artifact.BitrateBPS,
		ProbeStatus: artifact.ProbeStatus,
	}.Normalize()
	if metadata.ProbeStatus != domain.MediaProbePassed {
		return ProbeError{Reason: "music_artifact_probe_not_passed"}
	}
	if metadata.DurationMS <= 0 || metadata.DurationMS > int64(MaxMusicInputDurationSec)*1000 {
		return ProbeError{Reason: "music_artifact_duration"}
	}
	if !musicMIMEAndMetadataAllowed(artifact.MimeType, metadata) {
		return ProbeError{Reason: "music_artifact_format"}
	}
	return nil
}

func parseAndValidateAudio(raw []byte) (domain.ArtifactMediaMetadata, error) {
	var payload struct {
		ffprobeOutput
		Packets []struct {
			Duration string `json:"duration_time"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return failedMetadata(), ProbeError{Reason: "probe_json_invalid"}
	}
	if hasVideoStream(payload.Streams) {
		return failedMetadata(), ProbeError{Reason: "audio_video_stream_not_allowed"}
	}
	stream := firstAudioStream(payload.Streams)
	if stream == nil {
		return failedMetadata(), ProbeError{Reason: "audio_stream_missing"}
	}
	metadata := domain.ArtifactMediaMetadata{
		Codec:       normalizeToken(stream.CodecName),
		Container:   allowedAudioContainer(payload.Format.FormatName),
		DurationMS:  audioDurationMillis(firstNonZero(stream.Duration, payload.Format.Duration)),
		BitrateBPS:  firstPositive(parseInt64(stream.BitRate), parseInt64(payload.Format.BitRate)),
		ProbeStatus: domain.MediaProbeFailed,
	}.Normalize()
	// Streamed MP3/AAC/WAV often omit duration because stdin is not seekable.
	// Use actual packet duration rather than a filename or declared header size.
	if len(payload.Packets) > 0 {
		seconds := 0.0
		for _, packet := range payload.Packets {
			d, err := strconv.ParseFloat(packet.Duration, 64)
			if err != nil || math.IsNaN(d) || math.IsInf(d, 0) || d <= 0 {
				return failedMetadata(), ProbeError{Reason: "audio_packet_duration_invalid"}
			}
			seconds += d
		}
		metadata.DurationMS = int64(math.Round(seconds * 1000))
	}
	if metadata.Container == "" {
		return metadata, ProbeError{Reason: "audio_container_not_allowed"}
	}
	if metadata.DurationMS <= 0 {
		return metadata, ProbeError{Reason: "audio_duration_missing"}
	}
	if metadata.DurationMS > int64(MaxMusicInputDurationSec)*1000 {
		return metadata, ProbeError{Reason: "audio_duration_exceeded"}
	}
	if !audioCodecAllowedForContainer(metadata.Codec, metadata.Container) {
		return metadata, ProbeError{Reason: "audio_codec_not_allowed"}
	}
	metadata.ProbeStatus = domain.MediaProbePassed
	return metadata, nil
}

func firstAudioStream(streams []ffprobeStream) *ffprobeStream {
	for i := range streams {
		if strings.EqualFold(strings.TrimSpace(streams[i].CodecType), "audio") {
			return &streams[i]
		}
	}
	return nil
}

func hasVideoStream(streams []ffprobeStream) bool {
	for _, stream := range streams {
		if strings.EqualFold(strings.TrimSpace(stream.CodecType), "video") {
			return true
		}
	}
	return false
}

func allowedAudioContainer(formatName string) string {
	for _, token := range strings.Split(formatName, ",") {
		switch normalizeToken(token) {
		case "mp3":
			return "mp3"
		case "aac", "adts":
			return "aac"
		case "wav", "wave":
			return "wav"
		}
	}
	return ""
}

func audioDurationMillis(value string) int64 {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0
	}
	return int64(math.Round(seconds * 1000))
}

func audioCodecAllowedForContainer(codec, container string) bool {
	codec = normalizeToken(codec)
	switch container {
	case "mp3":
		return codec == "mp3"
	case "aac":
		return codec == "aac"
	case "wav":
		return strings.HasPrefix(codec, "pcm_") || codec == "pcm"
	default:
		return false
	}
}

func musicMIMEAndMetadataAllowed(mimeType string, metadata domain.ArtifactMediaMetadata) bool {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	codec := normalizeToken(metadata.Codec)
	container := normalizeToken(metadata.Container)
	switch mimeType {
	case "audio/mpeg":
		return container == "mp3" && codec == "mp3"
	case "audio/aac":
		return container == "aac" && codec == "aac"
	case "audio/wav", "audio/x-wav":
		return container == "wav" && audioCodecAllowedForContainer(codec, container)
	default:
		return false
	}
}
