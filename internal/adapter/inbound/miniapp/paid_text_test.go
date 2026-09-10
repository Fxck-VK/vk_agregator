package miniapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	miniappinbound "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/textgeneration"
)

func TestPaidChatModelsReserveOnceAndKeepPrice(t *testing.T) {
	for _, id := range []string{"gpt_5_5", "claude_opus_4_7", "gemini_3_1_pro", "claude_opus_4_8", "gpt_5_6_terra", "gpt_6_astra", "claude_opus_5", "gemini_3_7_flash", "claude_fable_5_1", "claude_fable_5", "gemini_3_6_flash"} {
		t.Run(id, func(t *testing.T) {
			fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
				cfg.TextModels = textgeneration.Models([]string{id}, mustStaticPricingCatalog())
			})
			fixture.createVKUserWithCredits(t, 777, 1000)
			var first uuid.UUID
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewBufferString(`{"prompt":"Synthetic","model_id":"`+id+`"}`))
				req.Header.Set("X-Launch-Params", devLaunchParams(777))
				req.Header.Set("X-Idempotency-Key", "paid-text-"+id)
				recorder := httptest.NewRecorder()
				fixture.handler.Routes().ServeHTTP(recorder, req)
				if recorder.Code != http.StatusCreated {
					t.Fatalf("status %d", recorder.Code)
				}
				var dto struct {
					ID uuid.UUID `json:"id"`
				}
				_ = json.Unmarshal(recorder.Body.Bytes(), &dto)
				job, err := fixture.jobRepo.GetByID(context.Background(), dto.ID)
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]int64{"gpt_5_5": 20, "claude_opus_4_7": 20, "gemini_3_1_pro": 10, "claude_opus_4_8": 25, "gpt_5_6_terra": 10, "gpt_6_astra": 35, "claude_opus_5": 25, "gemini_3_7_flash": 5, "claude_fable_5_1": 90, "claude_fable_5": 45, "gemini_3_6_flash": 5}[id]
				if job.CostReserved != expected || job.ResultMode != domain.ResultModeAccountHistory || len(job.PricingSnapshot) == 0 {
					t.Fatal("missing reserved quote")
				}
				if i == 0 {
					first = dto.ID
				} else if first != dto.ID {
					t.Fatal("retry duplicated Job")
				}
			}
			changed := httptest.NewRequest(http.MethodPost, "/miniapp/chat/messages", bytes.NewBufferString(`{"prompt":"Different intent","model_id":"`+id+`"}`))
			changed.Header.Set("X-Launch-Params", devLaunchParams(777))
			changed.Header.Set("X-Idempotency-Key", "paid-text-"+id)
			recorder := httptest.NewRecorder()
			fixture.handler.Routes().ServeHTTP(recorder, changed)
			if recorder.Code != http.StatusConflict {
				t.Fatal("changed message reused paid idempotency key")
			}
		})
	}
}

func TestAPIMartTextGenericJobsUseVerifiedPriceAndAvailability(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		fixture := newTestFixtureWithConfig("", nil, func(cfg *miniappinbound.Config) {
			if enabled {
				cfg.TextModels = textgeneration.Models([]string{"claude_fable_5_1"}, mustStaticPricingCatalog())
			}
		})
		fixture.createVKUserWithCredits(t, 777, 1000)
		var first uuid.UUID
		for _, path := range []string{"/miniapp/estimate", "/miniapp/jobs", "/miniapp/jobs"} {
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"operation":"text_generate","prompt":"Synthetic","model_id":"claude_fable_5_1"}`))
			req.Header.Set("X-Launch-Params", devLaunchParams(777))
			req.Header.Set("X-Idempotency-Key", "fable-generic")
			resp := httptest.NewRecorder()
			fixture.handler.Routes().ServeHTTP(resp, req)
			if !enabled {
				if resp.Code != http.StatusBadRequest {
					t.Fatalf("disabled model accepted: %d", resp.Code)
				}
				continue
			}
			if resp.Code != http.StatusOK && resp.Code != http.StatusCreated {
				t.Fatalf("%s status=%d body=%s", path, resp.Code, resp.Body.String())
			}
			var dto struct {
				ID           uuid.UUID `json:"id"`
				CostEstimate int64     `json:"cost_estimate"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &dto); err != nil {
				t.Fatal(err)
			}
			if dto.CostEstimate != 90 {
				t.Fatalf("incorrect quote: %d", dto.CostEstimate)
			}
			if path == "/miniapp/jobs" {
				job, err := fixture.jobRepo.GetByID(context.Background(), dto.ID)
				if err != nil {
					t.Fatal(err)
				}
				if job.CostReserved != 90 || len(job.PricingSnapshot) == 0 {
					t.Fatal("missing frozen reservation")
				}
				var params struct {
					Provider  domain.ProviderName `json:"provider"`
					ModelCode string              `json:"model_code"`
				}
				if err := json.Unmarshal(job.Params, &params); err != nil {
					t.Fatal(err)
				}
				if params.Provider != domain.ProviderAPIMart || params.ModelCode != "claude-fable-5.1" {
					t.Fatal("incorrect server route")
				}
				if first != uuid.Nil && first != job.ID {
					t.Fatal("duplicate job")
				}
				first = job.ID
			}
		}
	}
}
