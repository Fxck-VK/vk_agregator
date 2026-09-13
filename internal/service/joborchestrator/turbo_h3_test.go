package joborchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func turboH3Input(t *testing.T) joborchestrator.CreateJobInput {
	t.Helper()
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	price, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteMiniMaxH3, Resolution: "2k", DurationSec: 5})
	if err != nil {
		t.Fatal(err)
	}
	return joborchestrator.CreateJobInput{UserID: uuid.New(), Source: "miniapp", Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, IdempotencyKey: "h3-priced-job", PricingSnapshot: price, Params: json.RawMessage(`{"video_route_alias":"video_minimax_h3","prompt":"Synthetic scene","duration_sec":5,"resolution":"2k","aspect_ratio":"16:9"}`)}
}

func TestTurboH3CreateRejectsDifferentPriceDimensions(t *testing.T) {
	for _, params := range []json.RawMessage{nil, []byte(`{`), []byte(`{"video_route_alias":"video_minimax_h3","prompt":"Synthetic scene","duration_sec":15,"resolution":"2k"}`), []byte(`{"video_route_alias":"video_kling_3_0_turbo","prompt":"Synthetic scene","duration_sec":5,"resolution":"1080p"}`)} {
		f := newFixture()
		in := turboH3Input(t)
		in.Params = params
		job, err := f.orch.CreateJob(context.Background(), in)
		if !errors.Is(err, joborchestrator.ErrBackendPriceRequired) {
			t.Fatalf("job=%v err=%v want price binding rejection", job != nil, err)
		}
		assertNoJobReservationOrTask(t, f, in.UserID)
	}
}

func TestTurboH3ReplayRejectsChangedPrompt(t *testing.T) {
	f := newFixture()
	in := turboH3Input(t)
	// A low balance can park the original job; it must still bind the replay intent.
	if _, err := f.orch.CreateJob(context.Background(), in); err != nil && !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatal(err)
	}
	var params map[string]any
	_ = json.Unmarshal(in.Params, &params)
	params["prompt"] = "Different scene"
	in.Params, _ = json.Marshal(params)
	if _, err := f.orch.CreateJob(context.Background(), in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed intent replay err=%v want conflict", err)
	}
}
