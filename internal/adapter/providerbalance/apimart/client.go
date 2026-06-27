package apimart

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

const defaultBaseURL = "https://api.apimart.ai/v1"

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Checker struct {
	apiKey string
	base   string
	http   *http.Client
	now    func() time.Time
}

func New(cfg Config) *Checker {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
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
	return "apimart"
}

func (c *Checker) Check(ctx context.Context) (providerbalance.ProviderBalance, error) {
	return c.check(ctx, "/user/balance")
}

func (c *Checker) CheckToken(ctx context.Context) (providerbalance.ProviderBalance, error) {
	return c.check(ctx, "/balance")
}

func (c *Checker) check(ctx context.Context, path string) (providerbalance.ProviderBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("APIMart balance API request build failed: %w", err)
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
		return providerbalance.ProviderBalance{}, fmt.Errorf("APIMart balance API returned %d", resp.StatusCode)
	}

	var decoded balanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("decode APIMart balance response: %w", err)
	}
	if !decoded.Success {
		return providerbalance.ProviderBalance{}, fmt.Errorf("APIMart balance API success=false")
	}
	return providerbalance.ProviderBalance{
		Provider:      c.Name(),
		RemainBalance: decoded.RemainBalance,
		RemainCredits: decoded.RemainCredits,
		UsedBalance:   decoded.UsedBalance,
		UsedCredits:   decoded.UsedCredits,
		CheckedAt:     c.now(),
	}, nil
}

type balanceResponse struct {
	Success       bool    `json:"success"`
	RemainBalance float64 `json:"remain_balance"`
	RemainCredits float64 `json:"remain_credits"`
	UsedBalance   float64 `json:"used_balance"`
	UsedCredits   float64 `json:"used_credits"`
}

func transportError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("APIMart balance API request failed: context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("APIMart balance API request failed: context deadline exceeded")
	default:
		return fmt.Errorf("APIMart balance API request failed")
	}
}
