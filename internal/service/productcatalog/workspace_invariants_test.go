package productcatalog

import (
	"slices"
	"testing"
)

func TestWorkspaceDefaultAspectUsesPricedPairForDefaultQuality(t *testing.T) {
	controls := WorkspaceImage{
		DefaultQuality: "2K", AllowedAspectRatios: []string{"1:1", "9:16"},
		PriceByVariant: map[string]int64{"1K:1:1": 10, "2K:9:16": 30},
	}
	if got := workspaceImageDefaultAspect(controls); got != "9:16" {
		t.Fatalf("default must be priced for the default quality, got %q", got)
	}
	controls.PriceByVariant["2K:16:9"] = 40
	controls.AllowedAspectRatios = append(controls.AllowedAspectRatios, "16:9")
	if got := workspaceImageDefaultAspect(controls); got != "16:9" {
		t.Fatalf("prefer the usual aspect when this quality supports it, got %q", got)
	}
}

func TestWorkspaceFreeCategoryComesFromServerPrice(t *testing.T) {
	if slices.Contains(workspacePriceCategories([]string{"popular", "text", "free"}, false), "free") {
		t.Fatal("a declared category must not advertise a paid model as free")
	}
	if !slices.Contains(workspacePriceCategories([]string{"text"}, true), "free") {
		t.Fatal("the free server route must remain discoverable as free")
	}
}
