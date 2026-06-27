# Runway And DeepInfra Provider Monitoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the read-only Telegram provider monitoring bot with Runway organization credits and DeepInfra account balance.

**Architecture:** Keep all provider monitoring code under `internal/adapter/providerbalance/*` and wire it only from `cmd/provider-balance-bot`. Runway is a balance checker because its API returns `creditBalance`; DeepInfra is a balance checker because its billing checklist endpoint returns `stripe_balance`, where a negative value means ready-to-spend funds and a positive value means money owed. The bot remains read-only and does not call generation APIs, mutate billing, create jobs, or touch VK/Mini App/API flows.

**Tech Stack:** Go, existing `internal/service/providerbalance`, existing `internal/platform/config`, Runway REST API, DeepInfra REST API, Telegram Bot API already implemented.

---

## Verified API Contracts

Runway official docs:
- URL: `https://docs.dev.runwayml.com/api/#tag/Organization/paths/~1v1~1organization/get`
- Method: `GET`
- Runtime URL: `${RUNWAYML_BASE_URL}/organization`
- Default base URL: `https://api.dev.runwayml.com/v1`
- Auth header: `Authorization: Bearer ${RUNWAYML_API_SECRET}`
- Required version header: `X-Runway-Version: 2024-11-06`
- Success condition: HTTP 200 and JSON has integer `creditBalance`
- Response fields used by this feature:

```json
{
  "creditBalance": 1000
}
```

DeepInfra official docs:
- URL: `https://docs.deepinfra.com/api-reference/billing/get-checklist`
- Method: `GET`
- Runtime URL: `${DEEPINFRA_BALANCE_BASE_URL}/payment/checklist`
- Default balance base URL: `https://api.deepinfra.com`
- Auth header: `Authorization: Bearer ${DEEPINFRA_API_KEY}`
- Success condition: HTTP 200 and JSON has numeric `stripe_balance`
- Response fields used by this feature:

```json
{
  "stripe_balance": -25.5,
  "recent": 123,
  "limit": 500,
  "suspended": false,
  "overdue_invoices": 0
}
```

Rules:
- Treat DeepInfra current balance as `RemainBalance = -stripe_balance - recent`.
- This means prepaid funds are reduced by unbilled recent usage, and positive `stripe_balance` plus `recent` becomes negative balance/debt.
- Do not log or render raw billing address, payment method, raw payloads, tokens, auth headers, or request URLs.

## File Map

- Create: `internal/adapter/providerbalance/runway/client.go`
- Create: `internal/adapter/providerbalance/runway/client_test.go`
- Create: `internal/adapter/providerbalance/deepinfra/client.go`
- Create: `internal/adapter/providerbalance/deepinfra/client_test.go`
- Modify: `internal/service/providerbalance/service.go`
- Modify: `internal/service/providerbalance/service_test.go`
- Modify: `cmd/provider-balance-bot/main.go`
- Modify: `cmd/provider-balance-bot/main_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `.env.example`
- Modify: `scripts/deploy/check-prod-env.sh`
- Modify: `scripts/deploy/check-prod-env.ps1`
- Modify: `scripts/deploy/deploy-dev.sh`

## Task 1: Runway Balance Checker

**Files:**
- Create: `internal/adapter/providerbalance/runway/client_test.go`
- Create: `internal/adapter/providerbalance/runway/client.go`

- [ ] **Step 1: Write failing Runway adapter tests**

Create `internal/adapter/providerbalance/runway/client_test.go` with tests for:
- request method is `GET`;
- path is `/v1/organization` when base URL is `server.URL + "/v1"`;
- auth header is `Bearer test-secret`;
- `X-Runway-Version` is `2024-11-06`;
- parses `creditBalance` into `providerbalance.ProviderBalance.RemainCredits`;
- sets `Provider` to `runway`;
- rejects HTTP 401;
- rejects invalid JSON;
- returned errors do not contain API secret, bearer header, request URL, or raw response body.

Core test body:

```go
func TestBalanceCheckerRequestsOrganizationAndParsesCreditBalance(t *testing.T) {
	var seenMethod, seenPath, seenAuth, seenVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenVersion = r.Header.Get("X-Runway-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"creditBalance":1000,"usage":{"models":{}}}`))
	}))
	defer server.Close()

	checker := New(Config{APISecret: "test-secret", BaseURL: server.URL + "/v1"})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/v1/organization" {
		t.Fatalf("path = %q, want /v1/organization", seenPath)
	}
	if seenAuth != "Bearer test-secret" {
		t.Fatalf("Authorization = %q, want Bearer test-secret", seenAuth)
	}
	if seenVersion != "2024-11-06" {
		t.Fatalf("X-Runway-Version = %q, want 2024-11-06", seenVersion)
	}
	if balance.Provider != "runway" || balance.RemainCredits != 1000 {
		t.Fatalf("unexpected balance: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatal("CheckedAt must be set")
	}
}
```

Run:

```powershell
go test ./internal/adapter/providerbalance/runway -count=1
```

Expected before implementation: build failure for missing `New` and `Config`.

- [ ] **Step 2: Implement Runway checker**

Create `internal/adapter/providerbalance/runway/client.go`:

```go
package runway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vk-ai-aggregator/internal/service/providerbalance"
)

const (
	defaultBaseURL    = "https://api.dev.runwayml.com/v1"
	defaultAPIVersion = "2024-11-06"
)

type Config struct {
	APISecret  string
	BaseURL    string
	APIVersion string
	HTTPClient *http.Client
}

type Checker struct {
	apiSecret  string
	base       string
	apiVersion string
	http       *http.Client
	now        func() time.Time
}

func New(cfg Config) *Checker {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	apiVersion := strings.TrimSpace(cfg.APIVersion)
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Checker{
		apiSecret:  strings.TrimSpace(cfg.APISecret),
		base:       baseURL,
		apiVersion: apiVersion,
		http:       httpClient,
		now:        time.Now,
	}
}

func (c *Checker) Name() string { return "runway" }

func (c *Checker) Check(ctx context.Context) (providerbalance.ProviderBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/organization", nil)
	if err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("Runway organization API request build failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiSecret)
	req.Header.Set("X-Runway-Version", c.apiVersion)

	resp, err := c.http.Do(req)
	if err != nil {
		return providerbalance.ProviderBalance{}, transportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerbalance.ProviderBalance{}, fmt.Errorf("Runway organization API returned %d", resp.StatusCode)
	}

	var decoded organizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("decode Runway organization response: %w", err)
	}
	return providerbalance.ProviderBalance{
		Provider:      c.Name(),
		RemainCredits: float64(decoded.CreditBalance),
		CheckedAt:     c.now(),
	}, nil
}

type organizationResponse struct {
	CreditBalance int64 `json:"creditBalance"`
}

func transportError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("Runway organization API request failed: context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("Runway organization API request failed: context deadline exceeded")
	default:
		return fmt.Errorf("Runway organization API request failed")
	}
}
```

- [ ] **Step 3: Verify Runway checker**

Run:

```powershell
gofmt -w internal\adapter\providerbalance\runway\client.go internal\adapter\providerbalance\runway\client_test.go
go test ./internal/adapter/providerbalance/runway -count=1
```

Expected: pass.

## Task 2: Provider Balance Service Supports Runway And DeepInfra Balances

**Files:**
- Modify: `internal/service/providerbalance/service_test.go`
- Modify: `internal/service/providerbalance/service.go`

- [x] **Step 1: Write failing service tests**

Add tests to `internal/service/providerbalance/service_test.go`:
- `/balance runway` renders Runway credits only and does not call APIMart/PoYo;
- `/balance deepinfra` renders DeepInfra balance only and does not call APIMart/PoYo/Runway;
- `/balances` includes APIMart, PoYo, Runway and DeepInfra balance when all checkers are registered;
- `/help` includes `/balance runway` and `/balance deepinfra`;
- `CheckAndWarn` treats DeepInfra as a normal balance provider.

Core test body:

```go
func TestServiceHandleBalanceDeepInfraRendersOnlyDeepInfra(t *testing.T) {
	messenger := &recordingMessenger{}
	deepinfra := &stubChecker{name: "deepinfra", balances: []ProviderBalance{{
		Provider:      "deepinfra",
		RemainBalance: 25.5,
		UsedBalance:   123,
		CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
	}}}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainCredits: 50}}}
	svc := New([]Checker{apimart, deepinfra}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance deepinfra"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg, "DeepInfra", "РћСЃС‚Р°С‚РѕРє: 25.50 balance", "РСЃРїРѕР»СЊР·РѕРІР°РЅРѕ: 123.00 balance")
	if apimart.calls != 0 {
		t.Fatalf("DeepInfra balance command called APIMart %d times", apimart.calls)
	}
}
```

Run:

```powershell
go test ./internal/service/providerbalance -count=1
```

Expected before implementation: failing help assertions and missing Runway/DeepInfra command coverage.

- [x] **Step 2: Update help and keep generic balance command routing**

The existing generic `/balance <provider>` lookup supports Runway and DeepInfra once checkers are registered. Update `helpMessage()`:

```go
return "Команды:\n/balances\n/balance apimart\n/balance poyo\n/balance runway\n/balance deepinfra\n/help"
```

- [x] **Step 3: Verify service**

Run:

```powershell
gofmt -w internal\service\providerbalance\service.go internal\service\providerbalance\service_test.go
go test ./internal/service/providerbalance -count=1
```

Expected: pass.

## Task 3: DeepInfra Balance Checker

**Files:**
- Create: `internal/adapter/providerbalance/deepinfra/client_test.go`
- Create: `internal/adapter/providerbalance/deepinfra/client.go`

- [x] **Step 1: Write failing DeepInfra adapter tests**

Create `internal/adapter/providerbalance/deepinfra/client_test.go` with tests for:
- request method is `GET`;
- path is `/payment/checklist`;
- request URL does not contain the API key;
- auth header is `Bearer x`;
- parses `stripe_balance` and optional `recent` into `providerbalance.ProviderBalance.RemainBalance` as `-stripe_balance - recent`;
- parses optional `recent` into `UsedBalance`;
- sets `Provider` to `deepinfra`;
- sets `CheckedAt`;
- rejects HTTP 401;
- rejects invalid JSON;
- returned errors do not contain API key, bearer header, request URL, raw response body, billing address, or payment method values.

Core test body:

```go
func TestBalanceCheckerRequestsChecklistAndParsesStripeBalance(t *testing.T) {
	var seenMethod, seenPath, seenRawQuery, seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenRawQuery = r.URL.RawQuery
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stripe_balance":-25.5,"recent":123,"limit":500,"suspended":false}`))
	}))
	defer server.Close()

	checker := New(Config{APIKey: "x", BalanceBaseURL: server.URL})
	balance, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/payment/checklist" {
		t.Fatalf("path = %q, want /payment/checklist", seenPath)
	}
	if seenRawQuery != "" {
		t.Fatalf("query = %q, want empty", seenRawQuery)
	}
	if seenAuth != "Bearer x" {
		t.Fatalf("Authorization = %q, want Bearer x", seenAuth)
	}
	if balance.Provider != "deepinfra" || balance.RemainBalance != 25.5 || balance.UsedBalance != 123 {
		t.Fatalf("unexpected balance: %+v", balance)
	}
	if balance.CheckedAt.IsZero() {
		t.Fatal("CheckedAt must be set")
	}
}
```

Run:

```powershell
go test ./internal/adapter/providerbalance/deepinfra -count=1
```

Expected before implementation: build failure for missing `New` and `Config`.

- [x] **Step 2: Implement DeepInfra balance checker**

Create `internal/adapter/providerbalance/deepinfra/client.go`:

```go
package deepinfra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vk-ai-aggregator/internal/service/providerbalance"
)

const defaultBalanceBaseURL = "https://api.deepinfra.com"

type Config struct {
	APIKey         string
	BalanceBaseURL string
	HTTPClient     *http.Client
}

type Checker struct {
	apiKey string
	base   string
	http   *http.Client
	now    func() time.Time
}

func New(cfg Config) *Checker {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BalanceBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBalanceBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Checker{
		apiKey: strings.TrimSpace(cfg.APIKey),
		base:   baseURL,
		http:   httpClient,
		now:    time.Now,
	}
}

func (c *Checker) Name() string { return "deepinfra" }

func (c *Checker) Check(ctx context.Context) (providerbalance.ProviderBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/payment/checklist", nil)
	if err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("DeepInfra balance API request build failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return providerbalance.ProviderBalance{}, transportError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerbalance.ProviderBalance{}, fmt.Errorf("DeepInfra balance API returned %d", resp.StatusCode)
	}

	var decoded checklistResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("decode DeepInfra balance response: %w", err)
	}
	return providerbalance.ProviderBalance{
		Provider:      c.Name(),
		RemainBalance: -decoded.StripeBalance,
		UsedBalance:   decoded.Recent,
		CheckedAt:     c.now(),
	}, nil
}

type checklistResponse struct {
	StripeBalance float64 `json:"stripe_balance"`
	Recent        float64 `json:"recent"`
}

func transportError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("DeepInfra balance API request failed: context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("DeepInfra balance API request failed: context deadline exceeded")
	default:
		return fmt.Errorf("DeepInfra balance API request failed")
	}
}
```

- [x] **Step 3: Verify DeepInfra checker**

Run:

```powershell
gofmt -w internal\adapter\providerbalance\deepinfra\client.go internal\adapter\providerbalance\deepinfra\client_test.go
go test ./internal/adapter/providerbalance/deepinfra -count=1
```

Expected: pass.

## Task 4: Config And Bot Wiring

**Files:**
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/provider-balance-bot/main.go`
- Modify: `cmd/provider-balance-bot/main_test.go`
- Modify: `.env.example`

- [x] **Step 1: Write failing config and wiring tests**

Add config tests:
- bot enabled and `RUNWAY_PROVIDER_ENABLED=false` does not require `RUNWAYML_API_SECRET`;
- bot enabled and `RUNWAY_PROVIDER_ENABLED=true` requires `RUNWAYML_API_SECRET` and `RUNWAYML_BASE_URL`;
- bot enabled and `DEEPINFRA_BALANCE_PROVIDER_ENABLED=false` does not require `DEEPINFRA_API_KEY`;
- bot enabled and `DEEPINFRA_BALANCE_PROVIDER_ENABLED=true` requires `DEEPINFRA_API_KEY` and `DEEPINFRA_BALANCE_BASE_URL`;
- validation errors contain env names only, not fake secret values.

Add command tests:
- `buildProviderBalanceCheckers` returns APIMart + PoYo + Runway when PoYo and Runway are enabled;
- `buildProviderBalanceCheckers` returns APIMart + DeepInfra when `DeepInfraBalanceProviderEnabled` is true;
- `buildProviderBalanceCheckers` returns APIMart + PoYo + Runway + DeepInfra when all optional providers are enabled.

Run:

```powershell
go test ./internal/platform/config ./cmd/provider-balance-bot -run "ProviderBalance|Runway|DeepInfra" -count=1
```

Expected before implementation: fail because Runway and DeepInfra monitoring are not wired.

- [x] **Step 2: Add config fields and env parsing**

Modify `Config` in `internal/platform/config/config.go`:

```go
DeepInfraBalanceProviderEnabled bool
DeepInfraBalanceBaseURL         string
```

Modify `Load()`:

```go
DeepInfraBalanceProviderEnabled: envBool("DEEPINFRA_BALANCE_PROVIDER_ENABLED", false),
DeepInfraBalanceBaseURL:         env("DEEPINFRA_BALANCE_BASE_URL", "https://api.deepinfra.com"),
```

Modify `validateProviderBalanceBotConfig()`:

```go
if c.RunwayProviderEnabled {
	if strings.TrimSpace(c.RunwayMLAPISecret) == "" {
		missing = append(missing, "RUNWAYML_API_SECRET")
	}
	if strings.TrimSpace(c.RunwayMLBaseURL) == "" {
		missing = append(missing, "RUNWAYML_BASE_URL")
	}
}
if c.DeepInfraBalanceProviderEnabled {
	if strings.TrimSpace(c.DeepInfraAPIKey) == "" {
		missing = append(missing, "DEEPINFRA_API_KEY")
	}
	if strings.TrimSpace(c.DeepInfraBalanceBaseURL) == "" {
		missing = append(missing, "DEEPINFRA_BALANCE_BASE_URL")
	}
}
```

- [x] **Step 3: Wire Runway and DeepInfra into bot checker construction**

Modify `cmd/provider-balance-bot/main.go` imports:

```go
deepinfrabalance "vk-ai-aggregator/internal/adapter/providerbalance/deepinfra"
runwaybalance "vk-ai-aggregator/internal/adapter/providerbalance/runway"
```

Modify `buildProviderBalanceCheckers`:

```go
if cfg.RunwayProviderEnabled {
	checkers = append(checkers, runwaybalance.New(runwaybalance.Config{
		APISecret: cfg.RunwayMLAPISecret,
		BaseURL:   cfg.RunwayMLBaseURL,
	}))
}
if cfg.DeepInfraBalanceProviderEnabled {
	checkers = append(checkers, deepinfrabalance.New(deepinfrabalance.Config{
		APIKey:         cfg.DeepInfraAPIKey,
		BalanceBaseURL: cfg.DeepInfraBalanceBaseURL,
	}))
}
```

- [x] **Step 4: Update `.env.example`**

Add near existing DeepInfra env block:

```env
DEEPINFRA_BALANCE_PROVIDER_ENABLED=false
DEEPINFRA_BALANCE_BASE_URL=https://api.deepinfra.com
```

Update provider balance comment:

```env
# Provider balance bot includes PoYo when POYO_PROVIDER_ENABLED=true,
# Runway when RUNWAY_PROVIDER_ENABLED=true, and DeepInfra balance when
# DEEPINFRA_BALANCE_PROVIDER_ENABLED=true.
```

- [x] **Step 5: Verify config and bot wiring**

Run:

```powershell
gofmt -w internal\platform\config\config.go internal\platform\config\config_test.go cmd\provider-balance-bot\main.go cmd\provider-balance-bot\main_test.go
go test ./internal/platform/config ./cmd/provider-balance-bot -run "ProviderBalance|Runway|DeepInfra" -count=1
```

Expected: pass.

## Task 5: Deploy Env Validation

**Files:**
- Modify: `scripts/deploy/check-prod-env.sh`
- Modify: `scripts/deploy/check-prod-env.ps1`
- Modify: `scripts/deploy/deploy-dev.sh`

- [x] **Step 1: Update production shell preflight**

In `scripts/deploy/check-prod-env.sh`, inside:

```bash
if is_true_value "$(get_value PROVIDER_BALANCE_BOT_ENABLED false)"; then
```

add:

```bash
  if is_true_value "$(get_value RUNWAY_PROVIDER_ENABLED false)"; then
    require_value RUNWAYML_API_SECRET "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
    require_value RUNWAYML_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
  fi
  if is_true_value "$(get_value DEEPINFRA_BALANCE_PROVIDER_ENABLED false)"; then
    require_value DEEPINFRA_API_KEY "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
    require_value DEEPINFRA_BALANCE_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
  fi
```

- [x] **Step 2: Update production PowerShell preflight**

In `scripts/deploy/check-prod-env.ps1`, inside provider balance validation, add:

```powershell
if (Is-TrueValue (Get-Value -Values $envValues -Name "RUNWAY_PROVIDER_ENABLED" -Default "false")) {
    foreach ($required in @("RUNWAYML_API_SECRET", "RUNWAYML_BASE_URL")) {
        Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
    }
}
if (Is-TrueValue (Get-Value -Values $envValues -Name "DEEPINFRA_BALANCE_PROVIDER_ENABLED" -Default "false")) {
    foreach ($required in @("DEEPINFRA_API_KEY", "DEEPINFRA_BALANCE_BASE_URL")) {
        Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
    }
}
```

- [x] **Step 3: Update DEV deploy validation**

In `scripts/deploy/deploy-dev.sh`, inside provider balance validation, add the same shell checks:

```bash
    if is_true_value "$(get_env_value RUNWAY_PROVIDER_ENABLED false)"; then
      for required in RUNWAYML_API_SECRET RUNWAYML_BASE_URL; do
        require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
      done
    fi
    if is_true_value "$(get_env_value DEEPINFRA_BALANCE_PROVIDER_ENABLED false)"; then
      for required in DEEPINFRA_API_KEY DEEPINFRA_BALANCE_BASE_URL; do
        require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
      done
    fi
```

- [x] **Step 4: Verify deploy script syntax**

Run:

```powershell
$gitBash='C:\Program Files\Git\bin\bash.exe'; & $gitBash -n scripts/deploy/check-prod-env.sh scripts/deploy/deploy-dev.sh
$errors=$null; [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path 'scripts\deploy\check-prod-env.ps1'), [ref]$null, [ref]$errors) > $null; if($errors -and $errors.Count -gt 0){ $errors | ForEach-Object { $_.ToString() }; exit 1 }
```

Expected: no output from Git Bash syntax check, no PowerShell parser errors.

## Task 6: Manual Verification

**Files:**
- No code edits.

- [x] **Step 1: Verify Runway organization balance without printing secret**

Run locally only:

```powershell
$headers = @{
  Authorization = "Bearer $env:RUNWAYML_API_SECRET"
  "X-Runway-Version" = "2024-11-06"
}
$resp = Invoke-RestMethod -Headers $headers -Uri "$($env:RUNWAYML_BASE_URL.TrimEnd('/'))/organization" -Method GET -TimeoutSec 20
[pscustomobject]@{
  ok = $true
  creditBalance_present = ($null -ne $resp.creditBalance)
} | ConvertTo-Json -Compress
```

Expected:
- output does not include `RUNWAYML_API_SECRET`;
- `creditBalance_present` is `true`.

- [x] **Step 2: Verify DeepInfra balance without printing token or raw URL**

Run locally only:

```powershell
$headers = @{ Authorization = "Bearer $env:DEEPINFRA_API_KEY" }
$base = $env:DEEPINFRA_BALANCE_BASE_URL.TrimEnd("/")
$resp = Invoke-RestMethod -Headers $headers -Uri "$base/payment/checklist" -Method GET -TimeoutSec 20
[pscustomobject]@{
  ok = $true
  stripe_balance_present = ($null -ne $resp.stripe_balance)
} | ConvertTo-Json -Compress
```

Expected:
- output does not include `DEEPINFRA_API_KEY`;
- output does not include request URL;
- `stripe_balance_present` is `true`.

- [ ] **Step 3: Verify Telegram command output**

Run the bot locally with provider monitoring enabled and send these commands in the configured Telegram admin thread:

```text
/balance runway
/balance deepinfra
/balances
```

Expected:
- `/balance runway` shows Runway credits;
- `/balance deepinfra` shows DeepInfra balance from `stripe_balance`;
- `/balances` includes APIMart, PoYo when enabled, Runway when enabled, and DeepInfra balance when enabled;
- no generation jobs are created.

Live note 2026-06-27:
- `/balance runway` and `/balance deepinfra` were sent through the Telegram command handler and returned ok.
- `/balances` was sent after correcting local `APIMART_BASE_URL` to `https://api.apimart.ai/v1`; APIMart, Runway and DeepInfra returned ok.
- PoYo returned HTTP 429 without `Retry-After`, so this live step remains blocked only on the external PoYo rate limit.

## Task 7: Required Checks

Run:

```powershell
go test ./internal/adapter/providerbalance/apimart ./internal/adapter/providerbalance/poyo ./internal/adapter/providerbalance/runway ./internal/adapter/providerbalance/deepinfra ./internal/adapter/delivery/telegram ./internal/service/providerbalance ./cmd/provider-balance-bot ./internal/platform/config
go test ./...
docker compose config
git diff --check
```

Run leak scan:

```powershell
git diff -U0 | rg -n "^\+.*(RUNWAYML_API_SECRET=.*[^=]|DEEPINFRA_API_KEY=.*[^=]|Authorization: Bearer [A-Za-z0-9_\-]{8,}|payment/(usage|checklist)/[A-Za-z0-9_\-]{8,}|private|raw payload)"
```

Expected:
- Go tests pass;
- compose config is valid;
- diff check has no whitespace errors;
- scan has no real secrets, token-bearing DeepInfra URLs, private URLs, or raw provider payloads in added lines;
- VK bot, Mini App, API, worker generation, billing ledger and provider routing remain unchanged.

## Done When

- `/balance runway` returns Runway `creditBalance` as credits when `RUNWAY_PROVIDER_ENABLED=true`.
- `/balance deepinfra` returns DeepInfra `stripe_balance` minus `recent` usage as current balance when `DEEPINFRA_BALANCE_PROVIDER_ENABLED=true`.
- `/balances` includes Runway and DeepInfra entries only when their gates are enabled.
- DeepInfra balance is treated like a normal low-balance warning source.
- Errors are sanitized and never include provider secrets, auth headers, token-bearing URLs, raw payloads or private URLs.
- Provider monitoring remains a separate `cmd/provider-balance-bot` runtime.
- No DB migrations are added.
- No generation provider adapters are modified.
- No billing ledger, VK handler, Mini App BFF, `cmd/api`, or worker job flow is changed.

## Final Cleanup Requirement

After this plan is implemented and verified, delete this temporary plan file before final handoff unless the user explicitly asks to keep it:

```text
docs/superpowers/plans/2026-06-27-runway-deepinfra-provider-monitoring.md
```

The final report must state whether the file was deleted and whether DEV/prod env on the VPS or GitHub deploy secrets were updated.
