package modelcontract

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEvidenceFilesMustExistAndMatchTheirHash(t *testing.T) {
	c := readyTextContract()
	var report strings.Builder
	for _, check := range c.Checks {
		fmt.Fprintf(&report, "## %s\nSynthetic test evidence.\n", check.Scenario)
	}
	data := []byte(report.String())
	for i := range c.Checks {
		c.Checks[i].EvidenceSHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	files := fstest.MapFS{"synthetic-verification.md": {Data: data}}
	if err := c.ValidateEvidenceFiles(files); err != nil {
		t.Fatal(err)
	}
	files["synthetic-verification.md"].Data = []byte("changed report")
	if err := c.ValidateEvidenceFiles(files); err == nil {
		t.Fatal("modified evidence accepted")
	}
	delete(files, "synthetic-verification.md")
	if err := c.ValidateEvidenceFiles(files); err == nil {
		t.Fatal("missing evidence accepted")
	}
	c.Checks[0].Evidence = "../private.md"
	if err := c.ValidateEvidenceFiles(files); err == nil {
		t.Fatal("path outside report directory accepted")
	}
}
