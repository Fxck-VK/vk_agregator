package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
)

type motionUploadProber struct{}

func (motionUploadProber) ProbeVideo(context.Context, []byte, int64) (domain.ArtifactMediaMetadata, error) {
	return domain.ArtifactMediaMetadata{Width: 320, Height: 240, DurationMS: 5100, Container: "mp4", Codec: "h264", BitrateBPS: 10000, ProbeStatus: domain.MediaProbePassed}, nil
}

func TestMotionUploadRequiresAuthAndReturnsOnlyOwnedArtifactAndDuration(t *testing.T) {
	f := newKlingVeoFixture(t)
	owner := f.createVKUserWithCredits(t, 777, 1000)
	h := miniappinbound.NewHandler(miniappinbound.Config{VideoRoutes: []miniappinbound.VideoRouteDTO{{Alias: string(domain.VideoRouteKling26Motion), Enabled: true}}}, miniappinbound.Deps{Users: f.userRepo, Billing: f.billing, Artifacts: f.artifactRepo, Objects: f.objects, VideoReferenceProber: motionUploadProber{}})
	for _, authenticated := range []bool{false, true} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "scene.mp4")
		if err != nil {
			t.Fatal(err)
		}
		// Valid MP4 ftyp brand header; metadata is supplied by the server probe stub.
		_, _ = part.Write([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'})
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/miniapp/video-artifacts", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if authenticated {
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
		}
		resp := httptest.NewRecorder()
		h.Routes().ServeHTTP(resp, req)
		if !authenticated {
			if resp.Code != 401 {
				t.Fatal("unauthenticated upload accepted")
			}
			continue
		}
		if resp.Code != 201 {
			t.Fatalf("upload failed %d %s", resp.Code, resp.Body.String())
		}
		var result struct {
			ArtifactID  string `json:"artifact_id"`
			DurationSec int    `json:"duration_sec"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.DurationSec != 6 || result.ArtifactID == "" {
			t.Fatal("missing server metadata")
		}
		var fields map[string]any
		_ = json.Unmarshal(resp.Body.Bytes(), &fields)
		if len(fields) != 2 {
			t.Fatal("private upload metadata leaked")
		}
		artifact, err := f.artifactRepo.GetByID(context.Background(), uuid.MustParse(result.ArtifactID))
		if err != nil {
			t.Fatal(err)
		}
		if artifact.OwnerAccountID != owner.EffectiveAccountID() || artifact.DurationMS != 5100 {
			t.Fatal("upload owner/probe metadata lost")
		}
	}
}
