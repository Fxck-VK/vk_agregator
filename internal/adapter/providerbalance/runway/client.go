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

func (c *Checker) Name() string {
	return "runway"
}

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
	defer func() {
		_ = resp.Body.Close()
	}()

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
