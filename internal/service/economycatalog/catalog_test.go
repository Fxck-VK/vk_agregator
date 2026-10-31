package economycatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedWebPolicyHasNoDrift(t *testing.T) {
	generated, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "platform", "src", "config", "economy.generated.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source, web any
	if err := json.Unmarshal(policyJSON, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(generated, &web); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(source, web) {
		t.Fatal("web economic policy differs: run python scripts/economy/export-web.py")
	}
}

func TestPlansAreExactApprovedOffers(t *testing.T) {
	want := []Plan{
		{Code: "lite", Name: "Lite", Amount: 19900, Stars: 400, PeriodDays: 7, Trial: true, NextPlanCode: "start"},
		{Code: "start", Name: "Start+", Amount: 54900, Stars: 1250, PeriodDays: 30},
		{Code: "pro", Name: "Pro", Amount: 99900, Stars: 2100, PeriodDays: 30},
		{Code: "ultima", Name: "Ultima", Amount: 199900, Stars: 4300, PeriodDays: 30},
		{Code: "elite", Name: "Elite", Amount: 499900, Stars: 11000, PeriodDays: 30},
	}
	if Version != 20261027 || StarNominalKopecks != 50 || SignupBonusStars != 60 || !reflect.DeepEqual(Plans(), want) {
		t.Fatalf("economic policy differs: version=%d plans=%+v", Version, Plans())
	}
	for _, plan := range want {
		got, ok := FindPlan(plan.Code)
		if !ok || got != plan {
			t.Fatalf("FindPlan(%q)=%+v,%v", plan.Code, got, ok)
		}
		if plan.NextPlanCode != "" {
			if _, ok := FindPlan(plan.NextPlanCode); !ok {
				t.Fatal("missing trial conversion plan")
			}
		}
	}
	if _, ok := FindPlan("annual"); ok {
		t.Fatal("unverified annual offer must not exist")
	}
	plans := Plans()
	plans[0].Stars = 1
	if Plans()[0].Stars != 400 {
		t.Fatal("caller changed canonical plans")
	}
}

func TestResearchFactsHaveCompleteDatedUniqueReferences(t *testing.T) {
	models := Models()
	if len(models) != 94 {
		t.Fatalf("model count=%d, want94", len(models))
	}
	urls, ids := map[string]bool{}, map[string]bool{}
	free := 0
	for _, model := range models {
		if urls[model.URL] || ids[model.ID] || model.ID == "" || model.SourceDate != "2026-10-27" || !strings.HasPrefix(model.URL, "https://study24.ai/chat/") || model.PricingSourceURL != "https://study24.ai/pricing" {
			t.Fatalf("invalid or duplicate source: %+v", model)
		}
		urls[model.URL], ids[model.ID] = true, true
		if model.FreeCategory {
			free++
			if model.MinimumStars != nil || model.Mapping.Status != "free_unverified" {
				t.Fatalf("free category became an admitted zero-price product: %+v", model)
			}
		} else if model.MinimumStars == nil || *model.MinimumStars <= 0 {
			t.Fatalf("missing paid public minimum: %+v", model)
		}
		if model.Unit == "request" && model.Mapping.Status == "exact" {
			t.Fatal("unknown text formula was invented")
		}
	}
	if free != 8 {
		t.Fatalf("free-category count=%d,want8", free)
	}
	omni, _ := FindModel("gemini_omni1")
	if !omni.PriceConflict || omni.CatalogCardStars == nil || *omni.CatalogCardStars != 40 || *omni.MinimumStars != 36 {
		t.Fatal("the public36/40 price-source conflict must remain explicit")
	}
	nano, _ := FindModel("nano_banana_pro")
	if nano.Mapping.Status != "exact" || nano.Mapping.Unit != "image" {
		t.Fatalf("verified per-image mapping missing: %+v", nano)
	}
	nano.Mapping.ProductIDs[0] = "changed"
	nano.Mapping.Qualities[0] = "changed"
	*nano.MinimumStars = 1
	fresh, _ := FindModel("nano_banana_pro")
	if fresh.Mapping.ProductIDs[0] != "nano_banana_pro" || fresh.Mapping.Qualities[0] != "1K" || *fresh.MinimumStars != 55 {
		t.Fatal("caller changed canonical nested facts")
	}
	models[0].Name = "changed"
	if Models()[0].Name == "changed" {
		t.Fatal("caller changed model list")
	}
}

func TestOnlyVerifiedImageVariantsReturnExactRetailRate(t *testing.T) {
	for _, quality := range []string{"1K", "2K", "4K"} {
		rate, ok := ImageRate("nano_banana_pro", quality)
		if !ok || rate != 55 {
			t.Fatalf("rate(%s)=%d,%v", quality, rate, ok)
		}
		// The observed four-image quote was220; count multiplies once.
		if rate*4 != 220 {
			t.Fatal("per-image units changed")
		}
	}
	for _, pair := range [][2]string{{"nano_banana_pro", "8K"}, {"gpt_image_2_5_sunburst", "1K-medium"}, {"seedream_5_0_flash", "2K"}, {"gpt_6_sol", ""}, {"unknown", "1K"}} {
		if _, ok := ImageRate(pair[0], pair[1]); ok {
			t.Fatalf("minimum became exact quote: %v", pair)
		}
	}
	start, _ := FindPlan("start")
	if start.Stars/55 != 22 || start.Stars%55 != 40 || start.PeriodDays != 30 {
		t.Fatal("whole-image capacity or period arithmetic changed")
	}
}

func TestPublicPolicyIsSafeAndDefensivelyCopied(t *testing.T) {
	policy := Current()
	if policy.SchemaVersion != 1 || policy.Version != Version || policy.CreditDenominationVersion != 2 || len(policy.OperationFacts) != 11 || len(policy.Gaps) == 0 {
		t.Fatalf("incomplete policy metadata: %+v", policy)
	}
	policy.Plans[0].Amount = 1
	policy.Gaps[0].Code = "changed"
	policy.OperationFacts[0].MinimumStars = 1
	if Current().Plans[0].Amount != 19900 || Current().Gaps[0].Code == "changed" || Current().OperationFacts[0].MinimumStars == 1 {
		t.Fatal("caller mutated policy")
	}
	raw, err := json.Marshal(Current())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"api_key", "secret", "payment_method", "provider_model", "authorization"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("public policy leaks %s", forbidden)
		}
	}
}
