package apimart

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func happyHorse10Request() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelHappyHorse10,
		Prompt:         "A small robot walks through a quiet garden",
		DurationSec:    5,
		Resolution:     "1080P",
		AspectRatio:    "16:9",
		IdempotencyKey: "provider-submit:happyhorse10",
	}
}

func happyHorse11Request() domain.ProviderRequest {
	req := happyHorse10Request()
	req.ModelCode = ModelHappyHorse11
	req.IdempotencyKey = "provider-submit:happyhorse11"
	return req
}

func skyReelsRequest() domain.ProviderRequest {
	return domain.ProviderRequest{
		Operation:      domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		ModelCode:      ModelSkyReelsV4Fast,
		Prompt:         "A serene forest at sunset with golden light filtering through the trees.",
		DurationSec:    5,
		Resolution:     "1080p",
		AspectRatio:    "16:9",
		IdempotencyKey: "provider-submit:skyreels",
	}
}

func TestHappyHorseSkyReelsDocumentedBodies(t *testing.T) {
	seedZero := 0
	tests := []struct {
		name  string
		req   domain.ProviderRequest
		media *domain.VideoMediaRequest
		want  map[string]any
	}{
		{
			name: "happyhorse 1.0 t2v keeps explicit seed zero",
			req:  happyHorse10Request(),
			media: &domain.VideoMediaRequest{
				Seed: &seedZero,
			},
			want: map[string]any{
				"model":      "happyhorse-1.0",
				"prompt":     "A small robot walks through a quiet garden",
				"resolution": "1080P",
				"size":       "16:9",
				"duration":   float64(5),
				"seed":       float64(0),
			},
		},
		{
			name: "happyhorse 1.0 edit omits inherited duration and ratio",
			req: func() domain.ProviderRequest {
				req := happyHorse10Request()
				req.Resolution = "720P"
				req.DurationSec = 11
				req.AspectRatio = "9:16"
				return req
			}(),
			media: &domain.VideoMediaRequest{
				ReferenceVideos: []domain.VideoReferenceVideo{{Type: domain.VideoReferenceVideoTypeEdit, URL: "https://cdn.test/source.mp4"}},
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{
					{Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/style.jpg"}},
				},
				Audio: &domain.VideoMediaAudio{Setting: domain.VideoAudioSettingOrigin},
			},
			want: map[string]any{
				"model":         "happyhorse-1.0",
				"prompt":        "A small robot walks through a quiet garden",
				"resolution":    "720P",
				"video_url":     "https://cdn.test/source.mp4",
				"image_urls":    []any{"https://cdn.test/style.jpg"},
				"audio_setting": "origin",
			},
		},
		{
			name: "happyhorse 1.1 reference images",
			req:  happyHorse11Request(),
			media: &domain.VideoMediaRequest{
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{
					{Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}},
				},
			},
			want: map[string]any{
				"model":      "happyhorse-1.1",
				"prompt":     "A small robot walks through a quiet garden",
				"resolution": "1080P",
				"size":       "16:9",
				"duration":   float64(5),
				"image_urls": []any{"https://cdn.test/a.png", "https://cdn.test/b.png"},
			},
		},
		{
			name: "skyreels i2v keyframes",
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.ModelCode = ModelSkyReelsV4Std
				req.Prompt = "The King summons a flying dragon. @image1 The dragon lowers."
				req.DurationSec = 8
				return req
			}(),
			media: &domain.VideoMediaRequest{
				StartFrame: &domain.VideoFrame{URL: "https://cdn.test/start.png"},
				EndFrame:   &domain.VideoFrame{URL: "https://cdn.test/end.png"},
				KeyFrames: []domain.VideoKeyFrame{
					{Tag: "@image1", URL: "https://cdn.test/key.png", TimeStampSec: intPtr(3)},
				},
			},
			want: map[string]any{
				"model":             "skyreels-v4-std",
				"prompt":            "The King summons a flying dragon. @image1 The dragon lowers.",
				"duration":          float64(8),
				"resolution":        "1080p",
				"prompt_optimizer":  true,
				"first_frame_image": "https://cdn.test/start.png",
				"end_frame_image":   "https://cdn.test/end.png",
				"mid_frame_images": []any{
					map[string]any{"tag": "@image1", "image_url": "https://cdn.test/key.png", "time_stamp": float64(3)},
				},
			},
		},
		{
			name: "skyreels omni reference video inherits duration",
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "The man from @image_1 imitates @video_1."
				req.DurationSec = 15
				return req
			}(),
			media: &domain.VideoMediaRequest{
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{
					{Tag: "@image_1", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/actor.png"}},
				},
				ReferenceVideos: []domain.VideoReferenceVideo{
					{Tag: "@video_1", Type: domain.VideoReferenceVideoTypeReference, URL: "https://cdn.test/motion.mp4", DurationSec: 10},
				},
			},
			want: map[string]any{
				"model":            "skyreels-v4-fast",
				"prompt":           "The man from @image_1 imitates @video_1.",
				"resolution":       "1080p",
				"prompt_optimizer": true,
				"ref_images":       []any{map[string]any{"tag": "@image_1", "type": "image", "image_urls": []any{"https://cdn.test/actor.png"}}},
				"ref_videos":       []any{map[string]any{"tag": "@video_1", "type": "reference", "video_url": "https://cdn.test/motion.mp4"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/videos/generations" || r.Header.Get("Idempotency-Key") != tc.req.IdempotencyKey {
					t.Fatalf("unexpected submit call %s %s", r.Method, r.URL.Path)
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("request mismatch\ngot  %#v\nwant %#v", got, tc.want)
				}
				if strings.Contains(mustJSON(t, got), "nsfw_check") || strings.Contains(mustJSON(t, got), "watermark") {
					t.Fatalf("unpriced moderation/watermark flags must not be sent: %#v", got)
				}
				_, _ = w.Write([]byte(`{"code":200,"data":[{"status":"submitted","task_id":"native-video-task"}]}`))
			}))
			defer srv.Close()

			p := New(Config{APIKey: "fixture-key", BaseURL: srv.URL, HTTPClient: srv.Client()})
			for range 2 {
				task, err := p.submitHappyHorseSkyReelsWithMedia(context.Background(), tc.req, tc.media)
				if err != nil {
					t.Fatal(err)
				}
				if task.ExternalID != "native-video-task" || task.ModelCode != tc.req.ModelCode {
					t.Fatalf("task=%+v", task)
				}
			}
			if posts.Load() != 1 {
				t.Fatalf("paid submits=%d, want 1", posts.Load())
			}
		})
	}
}

func TestHappyHorseSkyReelsOmitsNilSeed(t *testing.T) {
	req := happyHorse10Request()
	body, err := New(Config{}).happyHorseSkyReelsBody(context.Background(), req, &domain.VideoMediaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	raw := mustJSON(t, body)
	if strings.Contains(raw, `"seed"`) {
		t.Fatalf("nil seed must be omitted: %s", raw)
	}
}

func TestHappyHorseSkyReelsRejectsInvalidRequestsBeforeHTTP(t *testing.T) {
	cases := map[string]struct {
		req   domain.ProviderRequest
		media *domain.VideoMediaRequest
	}{
		"happyhorse 1.1 edit": {
			req:   happyHorse11Request(),
			media: &domain.VideoMediaRequest{ReferenceVideos: []domain.VideoReferenceVideo{{Type: domain.VideoReferenceVideoTypeEdit, URL: "https://cdn.test/source.mp4"}}},
		},
		"happyhorse first and references": {
			req: happyHorse10Request(),
			media: &domain.VideoMediaRequest{
				StartFrame:           &domain.VideoFrame{URL: "https://cdn.test/start.png"},
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/ref.png"}}},
			},
		},
		"happyhorse edit too many refs": {
			req: happyHorse10Request(),
			media: &domain.VideoMediaRequest{
				ReferenceVideos:      []domain.VideoReferenceVideo{{Type: domain.VideoReferenceVideoTypeEdit, URL: "https://cdn.test/source.mp4"}},
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/1.png", "https://cdn.test/2.png", "https://cdn.test/3.png", "https://cdn.test/4.png", "https://cdn.test/5.png", "https://cdn.test/6.png"}}},
			},
		},
		"happyhorse private reference": {
			req:   happyHorse10Request(),
			media: &domain.VideoMediaRequest{StartFrame: &domain.VideoFrame{URL: "https://127.0.0.1/start.png"}},
		},
		"happyhorse raw audio flag": {
			req: func() domain.ProviderRequest {
				req := happyHorse10Request()
				req.VideoAudio = true
				return req
			}(),
		},
		"skyreels i2v and omni": {
			req: skyReelsRequest(),
			media: &domain.VideoMediaRequest{
				StartFrame:           &domain.VideoFrame{URL: "https://cdn.test/start.png"},
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Tag: "@actor", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/a.png"}}},
			},
		},
		"skyreels missing tag in prompt": {
			req: skyReelsRequest(),
			media: &domain.VideoMediaRequest{
				KeyFrames: []domain.VideoKeyFrame{{Tag: "@missing", URL: "https://cdn.test/key.png"}},
			},
		},
		"skyreels too many keyframes": {
			req:   skyReelsRequest(),
			media: &domain.VideoMediaRequest{KeyFrames: sevenKeyFrames()},
		},
		"skyreels keyframe timestamp boundary": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@image1 appears."
				return req
			}(),
			media: &domain.VideoMediaRequest{KeyFrames: []domain.VideoKeyFrame{{Tag: "@image1", URL: "https://cdn.test/key.png", TimeStampSec: intPtr(0)}}},
		},
		"skyreels image subject too many groups": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@a @b @c @d"
				return req
			}(),
			media: &domain.VideoMediaRequest{ReferenceImageGroups: []domain.VideoReferenceImageGroup{
				{Tag: "@a", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/a.png"}},
				{Tag: "@b", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/b.png"}},
				{Tag: "@c", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/c.png"}},
				{Tag: "@d", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/d.png"}},
			}},
		},
		"skyreels grid must be one image": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@grid shows a recipe."
				return req
			}(),
			media: &domain.VideoMediaRequest{ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Tag: "@grid", Type: domain.VideoReferenceImageTypeGrid, URLs: []string{"https://cdn.test/a.png", "https://cdn.test/b.png"}}}},
		},
		"skyreels reference video too long": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@video1 continues."
				return req
			}(),
			media: &domain.VideoMediaRequest{ReferenceVideos: []domain.VideoReferenceVideo{{Tag: "@video1", Type: domain.VideoReferenceVideoTypeReference, URL: "https://cdn.test/motion.mp4", DurationSec: 11}}},
		},
		"skyreels extend with images": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@video1 and @actor"
				return req
			}(),
			media: &domain.VideoMediaRequest{
				ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Tag: "@actor", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/a.png"}}},
				ReferenceVideos:      []domain.VideoReferenceVideo{{Tag: "@video1", Type: domain.VideoReferenceVideoTypeExtend, URL: "https://cdn.test/source.mp4", DurationSec: 8}},
			},
		},
		"skyreels audio on grid": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Prompt = "@grid speaks."
				return req
			}(),
			media: &domain.VideoMediaRequest{ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Tag: "@grid", Type: domain.VideoReferenceImageTypeGrid, URLs: []string{"https://cdn.test/grid.png"}, AudioURL: "https://cdn.test/audio.mp3"}}},
		},
		"skyreels unsupported resolution": {
			req: func() domain.ProviderRequest {
				req := skyReelsRequest()
				req.Resolution = "4k"
				return req
			}(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			defer srv.Close()
			_, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()}).submitHappyHorseSkyReelsWithMedia(context.Background(), tc.req, tc.media)
			var pe *Error
			if !errors.As(err, &pe) || pe.Class != domain.ProviderErrInvalidRequest || calls.Load() != 0 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestHappyHorseSkyReelsBoundaryCases(t *testing.T) {
	req := skyReelsRequest()
	req.Prompt = "@one @two @three"
	req.DurationSec = 15
	media := &domain.VideoMediaRequest{
		ReferenceImageGroups: []domain.VideoReferenceImageGroup{
			{Tag: "@one", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/1a.png", "https://cdn.test/1b.png", "https://cdn.test/1c.png", "https://cdn.test/1d.png", "https://cdn.test/1e.png"}},
			{Tag: "@two", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/2a.png"}},
			{Tag: "@three", Type: domain.VideoReferenceImageTypeImage, URLs: []string{"https://cdn.test/3a.png"}},
		},
	}
	if err := validateHappyHorseSkyReelsRequestWithMedia(req, media); err != nil {
		t.Fatalf("accepted SkyReels max ref subject boundary rejected: %v", err)
	}

	req = happyHorse10Request()
	req.DurationSec = 3
	media = &domain.VideoMediaRequest{ReferenceImageGroups: []domain.VideoReferenceImageGroup{{Type: domain.VideoReferenceImageTypeImage, URLs: []string{
		"https://cdn.test/1.png", "https://cdn.test/2.png", "https://cdn.test/3.png",
		"https://cdn.test/4.png", "https://cdn.test/5.png", "https://cdn.test/6.png",
		"https://cdn.test/7.png", "https://cdn.test/8.png", "https://cdn.test/9.png",
	}}}}
	if err := validateHappyHorseSkyReelsRequestWithMedia(req, media); err != nil {
		t.Fatalf("accepted HappyHorse nine-reference boundary rejected: %v", err)
	}
}

func intPtr(value int) *int { return &value }

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func sevenKeyFrames() []domain.VideoKeyFrame {
	frames := make([]domain.VideoKeyFrame, 7)
	for i := range frames {
		frames[i] = domain.VideoKeyFrame{Tag: "@image", URL: "https://cdn.test/key.png"}
	}
	return frames
}
