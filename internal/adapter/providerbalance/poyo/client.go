package poyo

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

const defaultBaseURL = "https://api.poyo.ai"

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
	return "poyo"
}

func (c *Checker) Check(ctx context.Context) (providerbalance.ProviderBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/user/balance", nil)
	if err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("PoYo balance API request build failed")
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
		return providerbalance.ProviderBalance{}, fmt.Errorf("PoYo balance API returned %d", resp.StatusCode)
	}

	var decoded balanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return providerbalance.ProviderBalance{}, fmt.Errorf("decode PoYo balance response: %w", err)
	}
	if decoded.Code != 200 {
		return providerbalance.ProviderBalance{}, fmt.Errorf("PoYo balance API code=%d", decoded.Code)
	}
	return providerbalance.ProviderBalance{
		Provider:      c.Name(),
		RemainCredits: decoded.Data.CreditsAmount,
		CheckedAt:     c.now(),
	}, nil
}

type balanceResponse struct {
	Code int `json:"code"`
	Data struct {
		CreditsAmount float64 `json:"credits_amount"`
	} `json:"data"`
}

func transportError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("PoYo balance API request failed: context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("PoYo balance API request failed: context deadline exceeded")
	default:
		return fmt.Errorf("PoYo balance API request failed")
	}
}
