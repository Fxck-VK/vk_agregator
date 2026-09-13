package worker

import (
	"encoding/json"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestPaidImageSnapshotCannotBypassBindingWithMalformedParams(t *testing.T) {
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: "gpt_image_2_5_flare", Quality: "1K-medium"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(snapshot)
	for _, params := range []json.RawMessage{nil, json.RawMessage(`{`), json.RawMessage(`null`), json.RawMessage(`{}`)} {
		job := &domain.Job{OperationType: domain.OperationImageGenerate, Modality: domain.ModalityImage, CostReserved: snapshot.InternalCredits, PricingSnapshot: raw, Params: params}
		if validatePaidImageJob(job, promptParams{}) == nil {
			t.Fatal("invalid execution metadata bypassed priced image validation")
		}
	}
}
