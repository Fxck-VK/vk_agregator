// Package legal embeds reviewed public templates, never operator secrets or PII.
package legal

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//go:embed catalog.json
var source []byte

type Section struct {
	Title      string     `json:"title"`
	Paragraphs []string   `json:"paragraphs"`
	Table      [][]string `json:"table"`
}
type Document struct {
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	TitleEN  string    `json:"title_en"`
	Sections []Section `json:"sections"`
}
type sourceCatalog struct {
	Version          string     `json:"version"`
	MarketingVersion string     `json:"marketing_version"`
	Date             string     `json:"date"`
	RevisionHash     string     `json:"revision_hash"`
	Documents        []Document `json:"documents"`
}
type Catalog struct {
	Published         bool       `json:"published"`
	Version           string     `json:"version"`
	Checksum          string     `json:"checksum"`
	MarketingVersion  string     `json:"marketing_version"`
	MarketingChecksum string     `json:"marketing_checksum"`
	Date              string     `json:"date"`
	Documents         []Document `json:"documents"`
	Snapshot          []byte     `json:"-"`
}

func digest(value string) string { s := sha256.Sum256([]byte(value)); return hex.EncodeToString(s[:]) }
func ValidateSource() error {
	var c sourceCatalog
	if err := json.Unmarshal(source, &c); err != nil {
		return err
	}
	var lines []string
	for _, d := range c.Documents {
		lines = append(lines, d.Slug, d.Title, d.TitleEN)
		for _, s := range d.Sections {
			lines = append(lines, s.Title)
			lines = append(lines, s.Paragraphs...)
			for _, r := range s.Table {
				lines = append(lines, r...)
			}
		}
	}
	if digest(strings.Join(lines, "\n")) != c.RevisionHash {
		return errors.New("legal: reviewed source checksum mismatch")
	}
	return nil
}
func New(name, inn string, publish bool) Catalog {
	if ValidateSource() != nil {
		panic("legal catalog source is not reviewed")
	}
	var c sourceCatalog
	if err := json.Unmarshal(source, &c); err != nil {
		panic("invalid legal catalog")
	}
	name = strings.TrimSpace(name)
	inn = strings.TrimSpace(inn)
	valid := name != "" && len(name) <= 256 && !strings.ContainsFunc(name, unicode.IsControl) && regexp.MustCompile(`^[0-9]{12}$`).MatchString(inn) && strings.Trim(inn, "0") != ""
	published := publish && valid
	checksum := digest(c.Version + "\n" + c.MarketingVersion + "\n" + c.Date + "\n" + c.RevisionHash + "\n" + name + "\n" + inn + "\n" + strconv.FormatBool(published))
	out := Catalog{Published: published, Version: c.Version, Checksum: checksum, MarketingVersion: c.MarketingVersion, MarketingChecksum: digest(checksum + "\n" + c.MarketingVersion), Date: c.Date, Documents: c.Documents}
	replace := strings.NewReplacer("{NAME}", name, "{INN}", inn)
	for i := range out.Documents {
		for j := range out.Documents[i].Sections {
			for k, p := range out.Documents[i].Sections[j].Paragraphs {
				out.Documents[i].Sections[j].Paragraphs[k] = replace.Replace(p)
			}
		}
	}
	out.Snapshot, _ = json.Marshal(out)
	return out
}
