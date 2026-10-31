package pricingcatalog

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestStudy24AdoptedStaticRetailQuotes(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		model, quality string
		amount         int64
	}{
		{PublicImageGPTImage2, "1K", 51}, {PublicImageGPTImage2, "4K", 51},
		{PublicImageGPTImage25Flare, "1K-medium", 52}, {PublicImageGPTImage25Flare, "4K-max", 355},
		{PublicImageSeedream50Lite, "2K", 30}, {PublicImageSeedream50Lite, "4K", 30},
		{PublicImageSeedream50Pro, "1.5K", 35}, {PublicImageSeedream50Pro, "2K", 35},
		{PublicImageQwenImage3, "1K", 30}, {PublicImageQwenImage3, "2K", 30},
		{PublicImageGrokImage20, "standard", 25},
	} {
		t.Run(tc.model+"/"+tc.quality, func(t *testing.T) {
			s, err := catalog.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: tc.model, Quality: tc.quality})
			if err != nil {
				t.Fatal(err)
			}
			if s.InternalCredits != tc.amount || s.InternalCreditCap != tc.amount || s.RetailPolicyVersion != 2026102802 || !s.Valid() {
				t.Fatalf("retail rule not applied: %+v, want%d", s, tc.amount)
			}
		})
	}
}

func TestStudy24ParameterAwareImagePriceSurvivesFloorChanges(t *testing.T) {
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, quality := range []string{"1K-medium", "4K-high"} {
		s, err := catalog.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: PublicImageGPTImage25Flare, Quality: quality})
		if err != nil {
			t.Fatal(err)
		}
		quoted, err := QuoteAPIMartImage(s, "9:16", 0)
		if err != nil {
			t.Fatal(err)
		}
		if quoted.InternalCredits != 52 || quoted.ImageAspectRatio != "9:16" {
			t.Fatalf("ratio overwrote retail: %+v", quoted)
		}
		batch, err := ScaleGPTImage25Quote(quoted, 4)
		if err != nil {
			t.Fatal(err)
		}
		if batch.InternalCredits != 208 || batch.InternalCreditCap != 208 || batch.ImageOutputCount != 4 || !batch.Valid() {
			t.Fatalf("batch retail wrong: %+v", batch)
		}
	}
	base, err := catalog.Snapshot(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: PublicImageSeedream50Pro, Quality: "2K"})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := QuoteAPIMartImage(base, "1:1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if refs.InternalCredits != 35 || refs.Floor.Amount <= base.Floor.Amount || refs.FloorAmountCap != refs.Floor.Amount {
		t.Fatalf("reference retail/floor wrong: %+v", refs)
	}
}

func TestStudy24CompatibleVideoRetailAndUnroundedCeiling(t *testing.T) {
	for _, tc := range []struct {
		model, mode, resolution string
		seconds                 int
		amount                  int64
	}{
		{"seedance_2_0", "", "480p", 5, 237},
		{"seedance_2_0", "", "720p", 5, 525},
		{"seedance_2_0", "", "720p", 10, 1050},
		{"wan_3_0", "", "1080p", 5, 1000},
		{"grok_imagine_1_5_video", "", "720p", 6, 150},
		{"kling_2_6", "", "720p", 5, 300},
		{"kling_2_6", "pro-sound", "1080p", 10, 300},
	} {
		t.Run(tc.model+"/"+tc.resolution, func(t *testing.T) {
			q, err := MediaVideoCandidateQuote(tc.model, tc.mode, tc.resolution, tc.seconds)
			if err != nil {
				t.Fatal(err)
			}
			if q.InternalCredits != tc.amount || q.RetailPolicyVersion != 2026102802 || !q.Valid() {
				t.Fatalf("video retail wrong: %+v want%d", q, tc.amount)
			}
		})
	}
	catalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		route               domain.VideoRouteAlias
		resolution, quality string
		seconds             int
		amount              int64
	}{
		{domain.VideoRouteKlingV3, "1080p", VideoQualityAudio, 10, 500},
		{domain.VideoRouteKling30Turbo, "1080p", "", 7, 350},
		{domain.VideoRouteMiniMaxH3, "768p", "", 5, 375},
	} {
		q, err := catalog.Snapshot(ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: tc.route, Resolution: tc.resolution, Quality: tc.quality, DurationSec: tc.seconds})
		if err != nil {
			t.Fatal(err)
		}
		if q.InternalCredits != tc.amount || q.RetailPolicyVersion != 2026102802 {
			t.Fatalf("static video retail wrong: %+v", q)
		}
	}
}

func TestStudy24OnlyCompatibleFixedSunoOperations(t *testing.T) {
	for _, tc := range []struct {
		action string
		amount int64
	}{{"lyrics", 20}, {"sounds", 60}, {"stems", 100}, {"stems_all", 100}, {"generate", 60}} {
		q, err := MusicCandidateQuote("suno_v6", tc.action, false)
		if err != nil {
			t.Fatal(err)
		}
		if q.InternalCredits != tc.amount || q.RetailPolicyVersion != 2026102802 {
			t.Fatalf("Suno operation retail wrong: %+v", q)
		}
	}
}

func TestPaidTrackSnapshotUsesFrozenContractInsteadOfCurrentRetailRules(t *testing.T) {
	s, err := MusicCandidateQuote("suno_v6", "generate", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{20261028, 20261029} {
		s.RetailPolicyVersion = version
		if !s.Valid() {
			t.Fatalf("frozen paid-track terms rejected for version %d", version)
		}
	}
	s.Key.AudioAction = "lyrics"
	if s.Valid() {
		t.Fatal("text operation accepted as guaranteed playable track")
	}
	s.Key.AudioAction = "generate"
	s.MusicPaidTrackCount = 2
	if s.Valid() {
		t.Fatal("unimplemented minimum track count admitted")
	}
}
