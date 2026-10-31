package economycatalog_test

import (
	"testing"

	"vk-ai-aggregator/internal/service/economycatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestResearchMappingsReferenceExistingPublicProducts(t *testing.T) {
	registry := providermodels.StaticRegistry()
	known := map[string]bool{}
	for _, model := range registry.TextAliases {
		known[model.PublicID] = true
	}
	for _, model := range registry.ImageModels {
		known[model.PublicID] = true
	}
	for _, model := range registry.AudioModels {
		known[model.PublicID] = true
	}
	for _, route := range registry.VideoRouteModels {
		known[string(route.Alias)] = true
	}
	for _, model := range providermodels.TextCandidates() {
		known[model.PublicID] = true
	}
	for _, model := range providermodels.MediaCandidates() {
		known[model.PublicID] = true
	}
	for _, model := range economycatalog.Models() {
		if !economycatalog.IsKnownMappingStatus(model.Mapping.Status) {
			t.Fatalf("unknown mapping status: %+v", model.Mapping)
		}
		for _, id := range model.Mapping.ProductIDs {
			if !known[id] {
				t.Errorf("%s references absent public product %s", model.ID, id)
			}
		}
	}
	if len(economycatalog.MissingOrAmbiguous()) != 93 {
		t.Fatal("unverified research operations were hidden or admitted")
	}
	if economycatalog.IsKnownMappingStatus("enabled") {
		t.Fatal("admission and economic mapping status conflated")
	}
}
