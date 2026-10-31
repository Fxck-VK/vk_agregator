package productcatalog_test

import (
	"testing"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestWholeCatalogueHasExecutableEconomicCoverage(t *testing.T) {
	c, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	report, err := productcatalog.AuditWorkspaceEconomy(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Models) < 108 || report.QuotesChecked < 1000 {
		t.Fatalf("incomplete audit: models%d quotes%d", len(report.Models), report.QuotesChecked)
	}
	for _, m := range report.Models {
		if m.State == "unpriced" {
			t.Fatalf("unpriced existing model: %s", m.ID)
		}
	}
}

func TestEconomyAuditRejectsChangedClientPricesAndUnpricedAdmission(t *testing.T) {
	c, _ := productcatalog.WorkspacePreviewCatalog()
	for i := range c.Items {
		if c.Items[i].ID == "nano_banana_pro" {
			c.Items[i].Operations[0].Image.PriceByVariant["1K:1:1"] = 1
		}
	}
	if _, err := productcatalog.AuditWorkspaceEconomy(c); err == nil {
		t.Fatal("UI/server price drift accepted")
	}
	c, _ = productcatalog.WorkspacePreviewCatalog()
	for i := range c.Items {
		if c.Items[i].ID == "gpt_4o_mini_tts" {
			c.Items[i].Operations[0].Enabled = true
		}
	}
	if _, err := productcatalog.AuditWorkspaceEconomy(c); err == nil {
		t.Fatal("unpriced TTS admitted")
	}
}
