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

func (c *Checker) Name() string {
	return "deepinfra"
}

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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerbalance.ProviderBalance{}, fmt.Errorf("DeepInfra balance API returned %d", resp.StatusCode)
	}

	var decoded checklistResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("decode DeepInfra balance response: %w", err)
	}
	if decoded.StripeBalance == nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("DeepInfra balance API missing stripe_balance")
	}
	return providerbalance.ProviderBalance{
		Provider:      c.Name(),
		RemainBalance: -*decoded.StripeBalance - decoded.Recent,
		UsedBalance:   decoded.Recent,
		CheckedAt:     c.now(),
	}, nil
}

type checklistResponse struct {
	StripeBalance *float64 `json:"stripe_balance"`
	Recent        float64  `json:"recent"`
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
