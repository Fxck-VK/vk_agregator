package pricingcatalog

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTextCandidatePricesMatchPublicEvidenceAndBoundedReply(t *testing.T) {
	raw, err := os.ReadFile("testdata/apimart-text-20260928.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence []struct {
		ID     string `json:"public_id"`
		Input  int64  `json:"input_micros_per_million"`
		Output int64  `json:"output_micros_per_million"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 21 {
		t.Fatal("incomplete source snapshot")
	}
	static, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evidence {
		q, err := TextCandidateQuote(e.ID)
		floor := (e.Input*8192 + e.Output*2048 + 999999) / 1000000
		retail := ((floor*3 + 24999) / 25000) * 5
		if err != nil || !q.Valid() || q.Floor.Amount != floor || q.InternalCredits != retail || q.InternalCreditCap != retail || q.TextInputTokenCap != 8192 || q.TextOutputTokenCap != 2048 {
			t.Fatalf("incorrect bounded quote %s: %v", e.ID, err)
		}
		if _, err := static.Snapshot(q.Key); err == nil {
			t.Fatal("candidate price entered static rollout")
		}
	}
	if _, err := TextCandidateQuote("unknown"); err == nil {
		t.Fatal("unknown model priced")
	}
}
