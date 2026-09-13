package videoreference

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

type fakeProber struct {
	metadata domain.ArtifactMediaMetadata
	err      error
	calls    int
	size     int64
}

func (p *fakeProber) ProbeVideo(_ context.Context, _ []byte, sizeBytes int64) (domain.ArtifactMediaMetadata, error) {
	p.calls++
	p.size = sizeBytes
	if p.err != nil {
		return domain.ArtifactMediaMetadata{ProbeStatus: domain.MediaProbeFailed}, p.err
	}
	return p.metadata, nil
}

func TestUploadStoresProbedMP4InputVideo(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	probe := &fakeProber{metadata: validMetadata(3501)}
	svc := New(repo, store, probe)
	userID := uuid.New()
	accountID := uuid.New()
	data := validMP4Bytes()

	artifact, err := svc.Upload(ctx, userID, accountID, data)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if probe.calls != 1 || probe.size != int64(len(data)) {
		t.Fatalf("probe calls=%d size=%d", probe.calls, probe.size)
	}
	if artifact.OwnerUserID != userID || artifact.OwnerAccountID != accountID || artifact.Kind != domain.ArtifactKindInput || artifact.MediaType != domain.MediaTypeVideo || artifact.Status != domain.ArtifactStatusReady {
		t.Fatalf("unexpected artifact ownership/shape: %+v", artifact)
	}
	if artifact.MimeType != "video/mp4" || artifact.DurationMS != 3501 || artifact.Codec != "h264" || artifact.Container != "mp4" || artifact.ProbeStatus != domain.MediaProbePassed {
		t.Fatalf("metadata not stored: %+v", artifact)
	}
	if got, ok := store.Get(artifact.StorageBucket, artifact.StorageKey); !ok || string(got) != string(data) {
		t.Fatalf("stored object mismatch ok=%v", ok)
	}
	seconds, err := Validate(artifact, accountID, MaxDurationSec)
	if err != nil || seconds != 4 {
		t.Fatalf("Validate = %d, %v; want 4, nil", seconds, err)
	}
}

func TestUploadRejectsMalformedNonMP4BeforeProbe(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	probe := &fakeProber{metadata: validMetadata(5000)}
	svc := New(repo, store, probe)

	_, err := svc.Upload(ctx, uuid.New(), uuid.New(), []byte("not an mp4"))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if probe.calls != 0 {
		t.Fatalf("probe called for malformed bytes")
	}
	if store.Len() != 0 {
		t.Fatalf("stored malformed upload")
	}
}

func TestUploadRejectsOversizedBeforeProbe(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	probe := &fakeProber{metadata: validMetadata(5000)}
	svc := New(repo, store, probe)

	_, err := svc.Upload(ctx, uuid.New(), uuid.New(), make([]byte, int(MaxBytes)+1))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if probe.calls != 0 || store.Len() != 0 {
		t.Fatalf("oversized upload reached probe/storage: calls=%d objects=%d", probe.calls, store.Len())
	}
}

func TestUploadRejectsTooShortAndTooLongProbeMetadata(t *testing.T) {
	for name, durationMS := range map[string]int64{
		"too_short": 2999,
		"too_long":  30001,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := memory.NewArtifactRepo()
			store := memory.NewObjectStore()
			probe := &fakeProber{metadata: validMetadata(durationMS)}
			svc := New(repo, store, probe)

			_, err := svc.Upload(ctx, uuid.New(), uuid.New(), validMP4Bytes())
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
			if probe.calls != 1 || store.Len() != 0 {
				t.Fatalf("duration rejection should happen after probe before storage: calls=%d objects=%d", probe.calls, store.Len())
			}
		})
	}
}

func TestUploadProbeFailureDoesNotStoreOrLeakRawError(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewArtifactRepo()
	store := memory.NewObjectStore()
	probe := &fakeProber{err: errors.New("raw path C:/secret/video.mp4 token=hidden")}
	svc := New(repo, store, probe)

	_, err := svc.Upload(ctx, uuid.New(), uuid.New(), validMP4Bytes())
	if !errors.Is(err, ErrProbeFailed) {
		t.Fatalf("err = %v, want ErrProbeFailed", err)
	}
	for _, forbidden := range []string{"C:/secret", "token=hidden"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("probe error leaked %q in %q", forbidden, err.Error())
		}
	}
	if store.Len() != 0 {
		t.Fatalf("probe failure stored object")
	}
}

func TestValidateRejectsWrongOwnerAndInvalidArtifactShape(t *testing.T) {
	owner := uuid.New()
	for name, tc := range map[string]struct {
		artifact *domain.Artifact
		account  uuid.UUID
		want     error
	}{
		"nil":         {artifact: nil, account: owner, want: ErrNotFound},
		"wrong_owner": {artifact: validArtifact(owner, 5000), account: uuid.New(), want: ErrNotFound},
		"non_input":   {artifact: mutateArtifact(validArtifact(owner, 5000), func(a *domain.Artifact) { a.Kind = domain.ArtifactKindOutput }), account: owner, want: ErrInvalid},
		"non_video":   {artifact: mutateArtifact(validArtifact(owner, 5000), func(a *domain.Artifact) { a.MediaType = domain.MediaTypeImage }), account: owner, want: ErrInvalid},
		"pending":     {artifact: mutateArtifact(validArtifact(owner, 5000), func(a *domain.Artifact) { a.Status = domain.ArtifactStatusPending }), account: owner, want: ErrInvalid},
		"unprobed":    {artifact: mutateArtifact(validArtifact(owner, 5000), func(a *domain.Artifact) { a.ProbeStatus = domain.MediaProbeUnknown }), account: owner, want: ErrInvalid},
		"too_large":   {artifact: mutateArtifact(validArtifact(owner, 5000), func(a *domain.Artifact) { a.SizeBytes = MaxBytes + 1 }), account: owner, want: ErrInvalid},
		"too_short":   {artifact: validArtifact(owner, 2999), account: owner, want: ErrInvalid},
		"too_long":    {artifact: validArtifact(owner, 30001), account: owner, want: ErrInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Validate(tc.artifact, tc.account, MaxDurationSec)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateUsesLegacyOwnerFallbackAndCeilsFractionalDuration(t *testing.T) {
	owner := uuid.New()
	artifact := validArtifact(owner, 3001)
	artifact.OwnerAccountID = uuid.Nil
	artifact.OwnerUserID = owner

	seconds, err := Validate(artifact, owner, MaxDurationSec)
	if err != nil {
		t.Fatalf("Validate legacy fallback: %v", err)
	}
	if seconds != 4 {
		t.Fatalf("seconds = %d, want 4", seconds)
	}
}

func validMetadata(durationMS int64) domain.ArtifactMediaMetadata {
	return domain.ArtifactMediaMetadata{
		Width:       1920,
		Height:      1080,
		DurationMS:  durationMS,
		Codec:       "h264",
		Container:   "mp4",
		BitrateBPS:  12_000_000,
		ProbeStatus: domain.MediaProbePassed,
	}
}

func validArtifact(owner uuid.UUID, durationMS int64) *domain.Artifact {
	return &domain.Artifact{
		ID:             uuid.New(),
		OwnerAccountID: owner,
		Kind:           domain.ArtifactKindInput,
		MediaType:      domain.MediaTypeVideo,
		MimeType:       "video/mp4",
		StorageBucket:  "artifacts",
		StorageKey:     "artifacts/ref.mp4",
		SizeBytes:      int64(len(validMP4Bytes())),
		Status:         domain.ArtifactStatusReady,
		Width:          1920,
		Height:         1080,
		DurationMS:     durationMS,
		Codec:          "h264",
		Container:      "mp4",
		BitrateBPS:     12_000_000,
		ProbeStatus:    domain.MediaProbePassed,
	}
}

func mutateArtifact(artifact *domain.Artifact, mutate func(*domain.Artifact)) *domain.Artifact {
	mutate(artifact)
	return artifact
}

func validMP4Bytes() []byte {
	data := []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x00,
		'i', 's', 'o', 'm',
		'm', 'p', '4', '2',
	}
	return append(data, []byte("video payload")...)
}
