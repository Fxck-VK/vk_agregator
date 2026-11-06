// Package releasepolicy projects the reviewed website release inventory.
// Its flags only restrict existing backend admission; they never grant it.
package releasepolicy

import (
	_ "embed"
	"encoding/json"
)

const Header = "X-Epifera-Release-View"

//go:embed features.json
var inventory []byte

var features = loadFeatures()

func loadFeatures() map[string]bool {
	var document struct {
		Version  int `json:"version"`
		Features []struct {
			ID         string `json:"id"`
			Production bool   `json:"production"`
		} `json:"features"`
	}
	if json.Unmarshal(inventory, &document) != nil || document.Version != 1 {
		panic("invalid release inventory")
	}
	result := make(map[string]bool, len(document.Features))
	for _, feature := range document.Features {
		if _, exists := result[feature.ID]; exists || feature.ID == "" {
			panic("invalid release feature ID")
		}
		result[feature.ID] = feature.Production
	}
	return result
}

// Profile trusts only the runtime environment. A client may reduce DEV to PROD.
func Profile(environment, requested string) string {
	if environment == "development" && requested != "prod" {
		return "dev"
	}
	return "prod"
}

// Enabled fails closed for unknown IDs and unknown profiles.
func Enabled(id, profile string) bool {
	production, known := features[id]
	return known && (production || profile == "dev")
}
