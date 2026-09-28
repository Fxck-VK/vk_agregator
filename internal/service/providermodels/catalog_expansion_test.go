package providermodels_test

import (
	"testing"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestCatalogExpansionCandidatesKeepAdmissionClosed(t *testing.T) {
	for _, id := range []string{"nano_banana", "grok_imagine_1_5_video", "kling_2_6", "seedance_2_0", "seedance_2_0_mini"} {
		t.Run(id, func(t *testing.T) {
			c, ok := providermodels.MediaCandidateByID(id)
			if !ok {
				t.Fatal("missing candidate")
			}
			contract := providermodels.DraftMediaContract(c)
			if c.CheckedAt != "2026-09-28" || contract.Status != "draft" || len(contract.Operations) == 0 || len(contract.Sources) == 0 {
				t.Fatal("missing dated draft evidence")
			}
			if contract.ValidateReady() == nil || providermodels.StaticRegistry().MediaCandidateAdmitted(id, "generate") {
				t.Fatal("unverified candidate admitted")
			}
			if c.Kind == "video" {
				if c.Capabilities.API.Video.Images.Support != providermodels.Supported || c.Capabilities.Application.Video.Images.Support != providermodels.Unsupported {
					t.Fatal("provider/application media statuses conflated")
				}
				if len(contract.Operations[0].Video.Variants) == 0 {
					t.Fatal("missing variants")
				}
			} else if *c.Capabilities.API.Image.Images.MaxCount != 14 || c.Capabilities.Application.Image.Images.Support != providermodels.Unsupported {
				t.Fatal("incorrect Nano input status")
			}
		})
	}
}
