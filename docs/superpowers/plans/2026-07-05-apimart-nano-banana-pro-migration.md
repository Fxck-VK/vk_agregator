# APIMart Nano Banana Pro Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move public image model `nano_banana_pro` from PoYo execution to the cheaper APIMart `gemini-3-pro-image-preview` channel, while keeping the public model ID, UX, optional reference images, and current customer-facing behavior stable.

**Architecture:** Public surfaces continue to use `nano_banana_pro`; only provider registry routing changes from PoYo to APIMart. VK/Mini App stay thin and continue creating jobs; worker/provider adapters remain the only layer that calls APIMart. `nano_banana_2` stays on PoYo because APIMart is not materially cheaper for that model.

**Tech Stack:** Go, APIMart `/v1/images/generations`, provider registry, product catalog, pricing catalog, VK/Mini App image job flow, worker provider routing.

---

## Source Docs

- Nano Banana Pro APIMart contract: `https://docs.apimart.ai/ru/api-reference/images/gemini-3-pro/generation`
- APIMart pricing: `https://apimart.ai/ru/pricing`
- Current PoYo Nano Banana Pro pricing reference: `https://poyo.ai/models/nano-banana-2-api`

## Verified Contract

- Public model: `nano_banana_pro`
- Target provider: APIMart
- Target provider model: `gemini-3-pro-image-preview`
- Official provider model is distinct: `gemini-3-pro-image-preview-official`
- Endpoint: `POST /v1/images/generations`
- Required body fields: `model`, `prompt`
- Body fields used by our adapter: `model`, `prompt`, `size`, `n`, `resolution`, `official_fallback`, `image_urls`
- References: optional `image_urls`, max 14 images, URL or Data URI, max 10 MB each
- Resolutions: `1K`, `2K`, `4K`
- Ratios: `auto`, `1:1`, `2:3`, `3:2`, `3:4`, `4:3`, `4:5`, `5:4`, `9:16`, `16:9`, `21:9`
- Pricing basis from APIMart standard channel: default `$0.04`, 4K `$0.05`
- Do not use official model by default.
- Do not set `official_fallback=true` in this migration.
- Do not change `nano_banana_2`; it remains PoYo-backed.

## File Map

- Modify: `internal/service/providermodels/registry.go`
  - Provider/model mapping and readiness for `nano_banana_pro` only.
- Modify: `internal/service/providermodels/registry_test.go`
  - Source-contract test for Nano Banana Pro and isolation checks proving Nano Banana 2 stays PoYo.
- Modify: `internal/adapter/provider/apimart/apimart_test.go`
  - Payload test for APIMart Nano Banana Pro contract.
- Modify: `internal/service/pricingcatalog/static_catalog.go`
  - Verify no change is needed if Nano Banana Pro APIMart floors are already present.
- Modify: `internal/service/pricingcatalog/static_catalog_test.go`
  - Assert Nano Banana Pro APIMart floor unit/prices stay correct.
- Modify: `internal/service/productcatalog/*_test.go`
  - Verify public catalog visibility for Nano Banana Pro depends on APIMart readiness.
- Modify: `internal/app/vkbot/module_test.go`
  - Verify VK image menu availability for Nano Banana Pro comes from APIMart readiness.
- Modify: `.env.example`, `.env.dev.example`, `.env.prod.example`
  - Keep APIMart flags/keys documented; remove any implication that Nano Banana Pro requires PoYo.
- Optional Modify: `scripts/deploy/prepare-dev-env.sh`, `scripts/deploy/test-dev-env.sh`
  - If current DEV env rendering still derives Nano Banana Pro from PoYo readiness, align only Nano Banana Pro with APIMart readiness.

## Phase 01: Replace Nano Banana Pro With APIMart

**Scope:** Only `nano_banana_pro`. Do not change `nano_banana_2`, GPT Image 2, Seedream, Runway, video routes, or PoYo video behavior.

- [x] **Step 1: Write failing registry test**

Modify `internal/service/providermodels/registry_test.go`:

```go
func TestRegistryNanoBananaProUsesAPIMartGemini3ProContract(t *testing.T) {
	registry := providermodels.StaticRegistry()

	model, ok := registry.PublicImageModel(modelcatalog.MiniAppImageNanoBananaPro)
	if !ok {
		t.Fatal("Nano Banana Pro image model missing")
	}
	if model.Provider != domain.ProviderAPIMart {
		t.Fatalf("Nano Banana Pro provider = %s, want %s", model.Provider, domain.ProviderAPIMart)
	}
	if model.ProviderModelID != providermodels.ProviderModelGemini3ProImage {
		t.Fatalf("Nano Banana Pro provider model = %q, want %q", model.ProviderModelID, providermodels.ProviderModelGemini3ProImage)
	}
	if model.Readiness.ProviderEnabledFlag != providermodels.ProviderFlagAPIMart {
		t.Fatalf("Nano Banana Pro readiness flag = %q, want %q", model.Readiness.ProviderEnabledFlag, providermodels.ProviderFlagAPIMart)
	}
	if !reflect.DeepEqual(model.Readiness.RequiredConfigKeys, []string{providermodels.ConfigKeyAPIMartAPIKey, providermodels.ConfigKeyAPIMartBaseURL}) {
		t.Fatalf("Nano Banana Pro readiness keys = %#v", model.Readiness.RequiredConfigKeys)
	}
	if !model.Limits.SupportsReferenceImage || model.Limits.MaxReferenceImages != 14 {
		t.Fatalf("Nano Banana Pro reference limits = %+v, want optional refs max 14", model.Limits)
	}

	nano2, ok := registry.PublicImageModel(modelcatalog.MiniAppImageNanoBanana2)
	if !ok {
		t.Fatal("Nano Banana 2 image model missing")
	}
	if nano2.Provider != domain.ProviderPoYo || nano2.ProviderModelID != providermodels.ProviderModelPoYoNanoBanana2 {
		t.Fatalf("Nano Banana 2 must stay on PoYo in this plan: %+v", nano2)
	}
}
```

- [x] **Step 2: Run focused failing test**

Run:

```powershell
go test ./internal/service/providermodels -run "NanoBananaPro|nano_banana_pro" -count=1
```

Expected before implementation: fail because `nano_banana_pro` still maps to PoYo.

- [x] **Step 3: Update registry mapping for Nano Banana Pro only**

Modify `internal/service/providermodels/registry.go` in `imageModels()`:

```go
imageModel(PublicImageNanoBananaPro, "Nano Banana Pro", domain.ProviderAPIMart, ProviderModelGemini3ProImage, FeatureImageNanoBananaPro, apimartReadiness(), 14),
```

Do not alter the `nano_banana_2` row.

- [x] **Step 4: Add APIMart adapter payload test for Nano Banana Pro**

Modify `internal/adapter/provider/apimart/apimart_test.go` by keeping or adding a focused test that submits `ModelGemini3ProImage` with `Params` containing `model_id:"nano_banana_pro"` and verifies:

```go
if seen.Model != ModelGemini3ProImage {
	t.Fatalf("model = %q, want %q", seen.Model, ModelGemini3ProImage)
}
if seen.Resolution != "4K" {
	t.Fatalf("resolution = %q, want 4K", seen.Resolution)
}
if len(seen.ImageURLs) != 1 || seen.ImageURLs[0] != "https://cdn.test/reference.png" {
	t.Fatalf("image_urls = %#v", seen.ImageURLs)
}
if _, ok := rawBody["official_fallback"]; ok {
	t.Fatalf("official_fallback must be omitted by default: %#v", rawBody)
}
```

- [x] **Step 5: Verify Nano Banana Pro pricing remains APIMart-backed**

Ensure `internal/service/pricingcatalog/static_catalog_test.go` contains:

```go
{modelID: PublicImageNanoBananaPro, quality: ImageQuality1K, want: 24, unit: FloorUnitAPIMartCredits},
{modelID: PublicImageNanoBananaPro, quality: ImageQuality2K, want: 30, unit: FloorUnitAPIMartCredits},
{modelID: PublicImageNanoBananaPro, quality: ImageQuality4K, want: 30, unit: FloorUnitAPIMartCredits},
```

Do not change `nano_banana_2` pricing.

- [x] **Step 6: Verify Nano Banana Pro focused tests**

Run:

```powershell
go test ./internal/adapter/provider/apimart ./internal/service/providermodels ./internal/service/pricingcatalog ./internal/service/productcatalog ./internal/app/vkbot -run "NanoBananaPro|nano_banana_pro|Gemini3Pro" -count=1
```

Expected: pass.

- [x] **Step 7: Commit Phase 01**

Run:

```powershell
git status --short
git add internal/service/providermodels/registry.go internal/service/providermodels/registry_test.go internal/adapter/provider/apimart/apimart_test.go internal/service/pricingcatalog/static_catalog_test.go
git commit -m "provider: route nano banana pro through apimart"
```

## Phase 02: Env And DEV Deploy Alignment

**Scope:** Nano Banana Pro readiness only. Do not derive Nano Banana 2 from APIMart.

- [x] **Step 1: Update env examples**

Ensure `.env.example`, `.env.dev.example`, and `.env.prod.example` describe:

```env
APIMART_PROVIDER_ENABLED=true
APIMART_BASE_URL=https://api.apimart.ai/v1
FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true
```

Do not state that Nano Banana Pro requires `POYO_PROVIDER_ENABLED`.

- [x] **Step 2: Update DEV env rendering if needed**

If `scripts/deploy/prepare-dev-env.sh` derives Nano Banana Pro feature flags from provider readiness, make only Nano Banana Pro follow APIMart readiness:

```bash
image_nano_banana_pro_enabled="${apimart_provider_enabled}"
```

Keep Nano Banana 2 PoYo-derived:

```bash
image_nano_banana_2_enabled="${poyo_provider_enabled}"
```

- [x] **Step 3: Run deploy env tests**

Run:

```powershell
bash scripts/deploy/test-dev-env.sh
bash scripts/deploy/test-env-parity.sh
```

Expected: pass or only documented parity warnings for secrets not available locally.

- [x] **Step 4: Commit env/deploy docs**

Run:

```powershell
git status --short
git add .env.example .env.dev.example .env.prod.example scripts/deploy/prepare-dev-env.sh scripts/deploy/test-dev-env.sh
git commit -m "deploy: derive nano banana pro flag from apimart"
```

## Phase 03: Full Verification

- [x] **Step 1: gofmt**

Run:

```powershell
gofmt -w internal\adapter\provider\apimart\apimart_test.go internal\service\providermodels\registry.go internal\service\providermodels\registry_test.go internal\service\pricingcatalog\static_catalog_test.go
```

- [x] **Step 2: Focused provider/catalog verification**

Run:

```powershell
go test ./internal/adapter/provider/apimart ./internal/adapter/provider/poyo ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/productcatalog ./internal/service/pricingcatalog -run "NanoBananaPro|nano_banana_pro|Gemini3Pro|NanoBanana2" -count=1
```

Expected: pass, and Nano Banana 2 remains PoYo.

- [x] **Step 3: VK/Mini App routing verification**

Run:

```powershell
go test ./internal/adapter/inbound/vk ./internal/adapter/inbound/miniapp ./internal/app/vkbot ./internal/app/miniapp -run "NanoBananaPro|ImageModels|model-catalog|Photo" -count=1
```

- [x] **Step 4: Worker/provider verification**

Run:

```powershell
go test ./internal/worker ./internal/adapter/provider/apimart ./internal/service/joborchestrator -run "NanoBananaPro|Gemini3Pro|Image" -count=1
```

- [x] **Step 5: Full backend verification**

Run:

```powershell
go test ./... -count=1
```

- [x] **Step 6: Final status**

Run:

```powershell
git status --short
```

Expected: clean after commits, or only intentional uncommitted env-local files.

## Risk Register

- **Pricing/execution mismatch:** Current state already has `nano_banana_pro` priced with APIMart floors while registry still routes it to PoYo. Phase 01 closes this mismatch.
- **Nano Banana 2 must not move:** APIMart is not materially cheaper for `nano_banana_2`; keep it on PoYo in this plan.
- **Official vs standard channel:** Standard APIMart `gemini-3-pro-image-preview` is cheaper; official variant is more expensive. Do not switch to `gemini-3-pro-image-preview-official` or `official_fallback=true` without a separate cost/quality decision.
- **PoYo collateral damage:** PoYo still owns Nano Banana 2, video routes, and Seedream 4.5. Do not remove PoYo provider readiness or API config globally.
- **Reference images:** APIMart target model accepts optional `image_urls` max 14. Text-only generation must omit `image_urls`; photo+text must include persisted artifact URLs resolved by worker.
- **DEV/PROD env drift:** `FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED` should depend on APIMart readiness in deploy rendering; production secrets need the same variable name to keep parity.
- **Public UX:** Model name and public ID remain unchanged, so VK/Mini App users should not see a new menu structure.

## Completion Checklist

- [x] `nano_banana_pro` registry provider is APIMart.
- [x] `nano_banana_pro` provider model is `gemini-3-pro-image-preview`.
- [x] `nano_banana_pro` requires APIMart readiness, not PoYo readiness.
- [x] `nano_banana_pro` keeps optional references with max 14.
- [x] `nano_banana_2` remains PoYo-backed.
- [x] Text-only APIMart requests omit `image_urls`.
- [x] Reference APIMart requests send `image_urls`.
- [x] Public model ID/name are unchanged.
- [x] PoYo video routes and Seedream 4.5 are unchanged.
- [x] Focused tests pass.
- [x] `go test ./... -count=1` passes or skipped failures are documented with unrelated root cause.
