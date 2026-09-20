package worker

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestMusicVoiceRejectsAACAndSampleRequiresImportedSource(t *testing.T) {
	for _, mime := range []string{"audio/mpeg", "audio/wav"} {
		if err := validateMusicUploadArtifactDuration(domain.MusicActionVoice, &domain.Artifact{MimeType: mime}); err != nil {
			t.Fatal(err)
		}
	}
	if validateMusicUploadArtifactDuration(domain.MusicActionVoice, &domain.Artifact{MimeType: "audio/aac"}) == nil {
		t.Fatal("voice accepted unsupported AAC")
	}
	if validateMusicSourceKind(domain.MusicActionSample, []musicSourceFact{{action: domain.MusicActionGenerate}}) == nil {
		t.Fatal("sample accepted generated source instead of upload")
	}
	if err := validateMusicSourceKind(domain.MusicActionSample, []musicSourceFact{{action: domain.MusicActionUpload}}); err != nil {
		t.Fatal(err)
	}
}
