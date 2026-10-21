// Package branding exposes public brand settings shared with the website.
package branding

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

//go:embed brand.json
var source []byte

var name = mustLoadName(source)

// Name is compiled into the binary; sending mail does not read the filesystem.
func Name() string { return name }

func mustLoadName(data []byte) string {
	name, err := parseName(data)
	if err != nil {
		panic(fmt.Errorf("branding: invalid shared configuration: %w", err))
	}
	return name
}

func parseName(data []byte) (string, error) {
	var settings struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", err
	}
	if settings.Name == "" || settings.Name != strings.TrimSpace(settings.Name) {
		return "", fmt.Errorf("name must be nonempty without surrounding whitespace")
	}
	if strings.ContainsFunc(settings.Name, unicode.IsControl) {
		return "", fmt.Errorf("name must not contain control characters")
	}
	return settings.Name, nil
}
