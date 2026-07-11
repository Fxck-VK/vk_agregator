package releasemanifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type trivyReport struct {
	SchemaVersion int             `json:"SchemaVersion"`
	ArtifactName  string          `json:"ArtifactName"`
	Results       json.RawMessage `json:"Results"`
}

type trivyResult struct {
	Vulnerabilities []struct {
		ID       string `json:"VulnerabilityID"`
		Severity string `json:"Severity"`
	} `json:"Vulnerabilities"`
}

// VerifyTrivyPolicy binds the scanner report to one immutable image and rejects
// every reported HIGH or CRITICAL vulnerability.
func VerifyTrivyPolicy(raw []byte, imageRef string) error {
	if _, err := digestFromImageRef(imageRef); err != nil {
		return err
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return fmt.Errorf("releasemanifest: Trivy report JSON is invalid: %w", err)
	}
	var report trivyReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("releasemanifest: Trivy report JSON is invalid: %w", err)
	}
	if report.SchemaVersion <= 0 || report.ArtifactName != imageRef {
		return errors.New("releasemanifest: Trivy report is not bound to the expected image digest")
	}
	trimmedResults := bytes.TrimSpace(report.Results)
	if len(trimmedResults) == 0 || trimmedResults[0] != '[' {
		return errors.New("releasemanifest: Trivy report results are missing")
	}
	var results []trivyResult
	if err := json.Unmarshal(trimmedResults, &results); err != nil {
		return fmt.Errorf("releasemanifest: Trivy report results are invalid: %w", err)
	}
	for _, result := range results {
		for _, vulnerability := range result.Vulnerabilities {
			if vulnerability.Severity == "HIGH" || vulnerability.Severity == "CRITICAL" {
				return fmt.Errorf("releasemanifest: Trivy policy rejected %s vulnerability %q", vulnerability.Severity, vulnerability.ID)
			}
		}
	}
	return nil
}
