package modelcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
)

func (c Contract) RequiredChecks() []string {
	var out []string
	for _, op := range c.Operations {
		for _, scenario := range []string{"adapter", "negative", "boundaries", "pricing", "job-lifecycle", "catalog", "live-output"} {
			out = append(out, op.ID+"/"+scenario)
		}
		for _, entry := range op.Inputs.entries() {
			if !entry.input.Enabled {
				continue
			}
			for _, format := range entry.input.Formats {
				out = append(out, op.ID+"/live-input/"+entry.name+"/"+format.Extension)
			}
		}
	}
	return out
}

func (c Contract) OutstandingChecks() []string {
	passed := map[string]bool{}
	for _, check := range c.Checks {
		if c.currentCheck(check) {
			passed[check.Scenario] = true
		}
	}
	var missing []string
	for _, scenario := range c.RequiredChecks() {
		if !passed[scenario] {
			missing = append(missing, scenario)
		}
	}
	return missing
}

func (c Contract) ValidateReady() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Status != "ready" {
		return fmt.Errorf("model contract %s is draft", c.PublicID)
	}
	seen := map[string]bool{}
	for _, check := range c.Checks {
		if seen[check.Scenario] || strings.TrimSpace(check.Scenario) == "" {
			return fmt.Errorf("duplicate/empty check scenario %q", check.Scenario)
		}
		seen[check.Scenario] = true
		if !c.currentCheck(check) {
			return fmt.Errorf("check %s has missing, failed or stale evidence", check.Scenario)
		}
	}
	if missing := c.OutstandingChecks(); len(missing) > 0 {
		return fmt.Errorf("model contract %s outstanding checks: %s", c.PublicID, strings.Join(missing, ", "))
	}
	return nil
}

func (c Check) current(digest string) bool {
	hash, err := hex.DecodeString(c.EvidenceSHA256)
	return c.Status == "passed" && fs.ValidPath(c.Evidence) && c.Evidence != "." && !strings.ContainsAny(c.Evidence, "\\:#") && err == nil && len(hash) == 32 && validDate(c.CheckedAt) && c.ContractDigest == digest
}

func (c Contract) currentCheck(check Check) bool {
	if !check.current(c.Digest()) {
		return false
	}
	for _, source := range c.Sources {
		if check.CheckedAt < source.CheckedAt {
			return false
		}
	}
	return true
}

// ValidateEvidenceFiles checks reviewed artifacts, not live provider behavior.
// Evidence references are relative to the candidate/approved contract directory.
func (c Contract) ValidateEvidenceFiles(files fs.FS) error {
	for _, check := range c.Checks {
		if !c.currentCheck(check) {
			return fmt.Errorf("invalid evidence reference for %s", check.Scenario)
		}
		data, err := fs.ReadFile(files, check.Evidence)
		if err != nil {
			return fmt.Errorf("missing report for %s: %w", check.Scenario, err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != check.EvidenceSHA256 {
			return fmt.Errorf("report changed for %s", check.Scenario)
		}
		found := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "## "+check.Scenario {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("report does not document scenario %s", check.Scenario)
		}
	}
	return nil
}
