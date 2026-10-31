package economycatalog

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestRetailPolicyBindsCompleteResearchAndDefendsCoverage(t *testing.T) {
	raw, err := os.ReadFile("../../../" + retail.ResearchPath)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != retail.ResearchSHA256 {
		t.Fatal("research changed without policy review")
	}
	rows := RetailCoverage()
	if len(rows) != 94 {
		t.Fatal("incomplete source coverage")
	}
	for _, row := range rows {
		if row.Status != "retained" && row.Status != "applied_compatible_variants" && row.Status != "workflow_tariff_ready" {
			t.Fatalf("unknown coverage status: %s", row.Status)
		}
	}
	rows[0].Name = "mutated"
	if RetailCoverage()[0].Name == "mutated" {
		t.Fatal("mutable policy")
	}
}

func TestRetailCeilingUsesWholeGenerationAndRejectsOverflow(t *testing.T) {
	for _, tc := range []struct {
		rate  string
		count int
		want  int64
	}{
		{"1.6861", 5, 9}, {"104.91160216", 5, 525}, {"25", 15, 375},
	} {
		got, ok := ceilRetailRate(tc.rate, tc.count)
		if !ok || got != tc.want {
			t.Fatalf("%s * %d = %d/%v, want %d", tc.rate, tc.count, got, ok, tc.want)
		}
	}
	for _, rate := range []string{"", "0", "-1", "NaN", "9223372036854775808"} {
		if _, ok := ceilRetailRate(rate, 1); ok {
			t.Fatalf("accepted %q", rate)
		}
	}
	if _, ok := VideoRetailRate("seedance_2_0", "reference_video", "720p", 5); ok {
		t.Fatal("unadmitted reference workflow")
	}
	if _, _, ok := MusicRetailRate("suno_v6_wild", "generate", false); ok {
		t.Fatal("inferred different music version")
	}
	if _, _, ok := MusicRetailRate("suno_v6", "generate", true); ok {
		t.Fatal("inferred Max price")
	}
	if _, ok := ImageRetailRate("gpt_image_2_5_sunburst", "1K-medium"); ok {
		t.Fatal("photoshoot price leaked to general model")
	}
}

func TestApplicationTariffsKeepWorkflowAndCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		model, quality string
		count          int
		want           int64
		ok             bool
	}{
		{"gpt_image_2_5_sunburst", "standard", 4, 100, true},
		{"nano_banana_2_1", "hd", 4, 140, true},
		{"qwen_image_2_1", "hd", 1, 0, false},
		{"gpt_image_2_5_sunburst", "standard", 5, 0, false},
		{"unknown", "standard", 1, 0, false},
	} {
		got, ok := PhotoshootPrice(tc.model, tc.quality, tc.count)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("photoshoot %+v: %d/%v", tc, got, ok)
		}
	}
	if SlidesPrice(false, false) != 450 || SlidesPrice(true, false) != 800 || SlidesPrice(true, true) != 860 {
		t.Fatal("slide workflow units changed")
	}
	for count, want := range map[int]int64{1: 120, 2: 120, 4: 120, 6: 180, 8: 240} {
		got, ok := SellerPrice(count)
		if !ok || got != want {
			t.Fatalf("seller %d: %d/%v", count, got, ok)
		}
	}
	for _, count := range []int{-1, 0, 3, 5, 7, 9} {
		if _, ok := SellerPrice(count); ok {
			t.Fatalf("seller accepts %d", count)
		}
	}
}
