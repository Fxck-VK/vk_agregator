package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videoreference"
)

type motionUploaderStub struct {
	calls    int
	artifact *domain.Artifact
	err      error
}

func (s *motionUploaderStub) Upload(_ context.Context, user, account uuid.UUID, _ []byte) (*domain.Artifact, error) {
	s.calls++
	if user != uuid.Nil || account != s.artifact.OwnerAccountID {
		return nil, domain.ErrNotFound
	}
	return s.artifact, s.err
}

func setupMotion(t *testing.T, h *Handler, owner uuid.UUID) (*webInputStoreStub, *motionUploaderStub, uuid.UUID) {
	t.Helper()
	h.cfg.Environment = "development"
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	route := conversationTestVideoRoute(domain.VideoRouteKling26Motion)
	route.Name = "Kling Motion Control"
	route.AllowedResolutions = []string{"std", "pro"}
	route.DefaultResolution = "std"
	route.AllowedDurationsSec = []int{3, 5, 6, 10, 11, 30}
	route.AllowedAspectRatios = nil
	route.DefaultAspectRatio = ""
	route.RequiresStartImage = true
	route.RequiresReferenceVideo = true
	h.cfg.VideoRoutes = []productcatalog.VideoRoute{route}
	store := &webInputStoreStub{}
	h.deps.InputArtifacts = store
	h.deps.InputObjects = store
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 512, 512)))
	photo, _ := store.SaveAccountInputArtifact(context.Background(), owner, domain.MediaTypeImage, "image/png", data.Bytes())
	video := &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeVideo, MimeType: "video/mp4", Status: domain.ArtifactStatusReady, SizeBytes: 1000, StorageBucket: "artifacts", StorageKey: "motion-reference", Width: 1280, Height: 720, DurationMS: 5042, Codec: "h264", Container: "mp4", BitrateBPS: 1000000, ProbeStatus: domain.MediaProbePassed}
	store.addArtifact(video, []byte("stored video"))
	uploader := &motionUploaderStub{artifact: video}
	h.deps.MotionVideoInputs = uploader
	return store, uploader, photo.ID
}

func TestMotionWebOwnedInputsPriceAndFailClosed(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	store, uploader, photo := setupMotion(t, h, owner)
	video := uploader.artifact
	body := map[string]any{"prompt": "Move the character naturally", "model_id": string(domain.VideoRouteKling26Motion), "resolution": "std", "character_orientation": "image", "keep_original_sound": false, "reference_artifact_ids": []uuid.UUID{photo}, "reference_video_artifact_id": video.ID}
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	rec := submitConversationMedia(t, h, sessions, owner, conv.ID, body)
	if rec.Code != 201 || len(jobs.inputs) != 1 {
		t.Fatalf("submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	in := jobs.inputs[0]
	var params map[string]any
	_ = json.Unmarshal(in.Params, &params)
	if in.PricingSnapshot.Key.DurationSec != 6 || params["duration_sec"] != float64(6) || params["keep_original_sound"] != false || !slices.Equal(in.InputArtifactIDs, []uuid.UUID{photo, video.ID}) {
		t.Fatal("trusted duration, sound or inputs lost")
	}
	quote := func(account uuid.UUID, orientation string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		h.Routes().ServeHTTP(r, authenticatedConversationRequest(t, "GET", "/web/v1/motion-reference-quote?resolution=std&character_orientation="+orientation+"&reference_video_artifact_id="+video.ID.String(), sessions, account))
		return r
	}
	rec = quote(owner, "image")
	var price struct {
		Credits     int64 `json:"credits"`
		DurationSec int   `json:"duration_sec"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &price)
	if rec.Code != 200 || price.Credits != in.PricingSnapshot.InternalCredits || price.DurationSec != 6 {
		t.Fatalf("quote status=%d body=%s", rec.Code, rec.Body.String())
	}
	if quote(uuid.New(), "image").Code != 404 {
		t.Fatal("foreign quote accepted")
	}
	for _, tc := range []struct {
		name    string
		change  func()
		restore func()
		status  int
	}{
		{"understated duration", func() { body["duration_sec"] = 3 }, func() { delete(body, "duration_sec") }, 400},
		{"foreign video", func() { video.OwnerAccountID = uuid.New() }, func() { video.OwnerAccountID = owner }, 404},
		{"missing image", func() { body["reference_artifact_ids"] = []uuid.UUID{} }, func() { body["reference_artifact_ids"] = []uuid.UUID{photo} }, 400},
		{"foreign image", func() { store.artifacts[photo].OwnerAccountID = uuid.New() }, func() { store.artifacts[photo].OwnerAccountID = owner }, 404},
		{"unprobed", func() { video.ProbeStatus = "pending" }, func() { video.ProbeStatus = domain.MediaProbePassed }, 400},
		{"long for image", func() { video.DurationMS = 10001 }, func() { video.DurationMS = 5042 }, 400},
		{"unsupported framing", func() { body["aspect_ratio"] = "16:9" }, func() { delete(body, "aspect_ratio") }, 400},
		{"unknown orientation", func() { body["character_orientation"] = "other" }, func() { body["character_orientation"] = "image" }, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			defer tc.restore()
			r := submitConversationMedia(t, h, sessions, owner, conv.ID, body)
			if r.Code != tc.status || len(jobs.inputs) != 1 {
				t.Fatalf("status=%d jobs=%d", r.Code, len(jobs.inputs))
			}
		})
	}
	video.DurationMS = 10001
	if quote(owner, "image").Code != 400 || quote(owner, "video").Code != 200 {
		t.Fatal("orientation duration boundary ignored")
	}
	video.DurationMS = 5042
	for _, env := range []string{"production", "prod", "", "test"} {
		h.cfg.Environment = env
		if len(h.conversationVideoRoutes()) != 0 {
			t.Fatalf("Motion exposed in %q", env)
		}
		if r := submitConversationMedia(t, h, sessions, owner, conv.ID, body); r.Code < 400 {
			t.Fatalf("Motion submitted in %q", env)
		}
	}
	h.cfg.Environment = "development"
	h.deps.MotionVideoInputs = nil
	if len(h.conversationVideoRoutes()) != 0 {
		t.Fatal("missing upload dependency advertised")
	}
}

func TestMotionUploadAuthenticationValidationAndCatalog(t *testing.T) {
	h, conversations, sessions, _, _ := newWebConversationMessageTestHandler(t)
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	_, uploader, _ := setupMotion(t, h, owner)
	upload := func(csrf bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		p, _ := m.CreateFormFile("file", "reference.mp4")
		_, _ = p.Write([]byte("synthetic bytes validated by service"))
		_ = m.Close()
		req := safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), "")
		req.URL.Path = "/web/v1/motion-video-input"
		req.Body = ioNopCloser(&b)
		req.Header.Set("Content-Type", m.FormDataContentType())
		if !csrf {
			req.Header.Del("X-CSRF-Token")
		}
		r := httptest.NewRecorder()
		h.Routes().ServeHTTP(r, req)
		return r
	}
	if r := upload(false); r.Code != 403 || uploader.calls != 0 {
		t.Fatal("CSRF bypass")
	}
	uploader.err = videoreference.ErrInvalid
	if upload(true).Code != 400 {
		t.Fatal("invalid video accepted")
	}
	uploader.err = nil
	if r := upload(true); r.Code != 201 {
		t.Fatalf("upload status %d", r.Code)
	}
	found := false
	for _, m := range h.workspaceModels().Items {
		if m.ID == string(domain.VideoRouteKling26Motion) {
			found = true
			op := m.Operations[0]
			if op.Video == nil || !op.Video.MotionControl || !op.Inputs.Images.Enabled || !op.Inputs.Video.Enabled || op.Inputs.Images.MaxCount != 1 || op.Inputs.Video.MaxCount != 1 {
				t.Fatal("Motion catalog lacks wired inputs")
			}
		}
	}
	if !found {
		t.Fatal("Motion missing from DEV catalog")
	}
	h.cfg.Environment = "production"
	before := uploader.calls
	if r := upload(true); r.Code < 400 || uploader.calls != before {
		t.Fatal("production upload allowed")
	}
}
