package providermodels_test

import (
	"os"
	"strings"
	"testing"
)

func TestPhase4CatalogSourcesDoNotDuplicateProviderModelIDs(t *testing.T) {
	files := []string{
		"../modelcatalog/catalog.go",
		"../videorouter/catalog.go",
	}
	forbidden := []string{
		"nano-banana-2",
		"gemini-3-pro-image-preview",
		"gpt-image-2",
		"MiniMax-Hailuo-2.3-Fast",
		"MiniMax-Hailuo-2.3",
		"kling-o3/standard",
		"seedance-2-fast",
		"gen4_turbo",
		"runway-gen-4.5",
		"mock-image",
		"mock-video",
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		source := string(data)
		for _, value := range forbidden {
			if strings.Contains(source, value) {
				t.Fatalf("%s still duplicates provider model id %q outside providermodels", file, value)
			}
		}
	}
}
