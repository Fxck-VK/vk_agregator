package config

import (
	"fmt"
	"net/url"
	"strings"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/providermodels"
)

// KIETextModels returns only explicitly enabled, operationally verified routes.
func (c Config) KIETextModels() []string {
	if !c.KIEProviderEnabled || !c.KIETextLimitsVerified || strings.TrimSpace(c.KIEAPIKey) == "" || strings.TrimSpace(c.KIEBaseURL) == "" {
		return nil
	}
	var out []string
	for _, model := range providermodels.PaidTextModels() {
		if model.Provider == domain.ProviderKIE && c.TextModelEnabled(model.PublicID) {
			out = append(out, model.ProviderModelID)
		}
	}
	return out
}

func (c Config) validateKIEText() error {
	selected := false
	for _, model := range providermodels.PaidTextModels() {
		if model.Provider == domain.ProviderKIE && c.TextModelEnabled(model.PublicID) {
			selected = true
		}
	}
	if !selected && !c.KIEProviderEnabled {
		return nil
	}
	if !c.KIEProviderEnabled || strings.TrimSpace(c.KIEAPIKey) == "" {
		return fmt.Errorf("config: KIE text requires KIE_PROVIDER_ENABLED and KIE_API_KEY")
	}
	u, err := url.Parse(c.KIEBaseURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "api.kie.ai" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("config: KIE_BASE_URL must be https://api.kie.ai")
	}
	if selected && !c.KIETextLimitsVerified {
		return fmt.Errorf("config: verify native output limits and usage before setting KIE_TEXT_LIMITS_VERIFIED=true")
	}
	return nil
}

// TextModelEnabled reads only an explicit server-side model flag.
func (c Config) TextModelEnabled(publicID string) bool {
	switch publicID {
	case providermodels.PublicTextGPT55:
		return c.FeatureTextGPT55Enabled
	case providermodels.PublicTextClaudeOpus47:
		return c.FeatureTextClaudeOpus47Enabled
	case providermodels.PublicTextGemini31Pro:
		return c.FeatureTextGemini31ProEnabled
	case providermodels.PublicTextClaudeOpus48:
		return c.FeatureTextClaudeOpus48Enabled
	case providermodels.PublicTextGPT56Terra:
		return c.FeatureTextGPT56TerraEnabled
	case providermodels.PublicTextGPT6Astra:
		return c.FeatureTextGPT6AstraEnabled
	case providermodels.PublicTextClaudeOpus5:
		return c.FeatureTextClaudeOpus5Enabled
	case providermodels.PublicTextGemini37Flash:
		return c.FeatureTextGemini37FlashEnabled
	case providermodels.PublicTextClaudeFable51:
		return c.FeatureTextClaudeFable51Enabled
	case providermodels.PublicTextClaudeFable5:
		return c.FeatureTextClaudeFable5Enabled
	case providermodels.PublicTextGemini36Flash:
		return c.FeatureTextGemini36FlashEnabled
	}
	return false
}

func (c Config) APIMartTextModels() []string {
	if !c.FeatureTextClaudeFable51Enabled || !c.APIMartProviderEnabled || !c.APIMartTextLimitsVerified || strings.TrimSpace(c.APIMartAPIKey) == "" || strings.TrimSpace(c.APIMartBaseURL) == "" {
		return nil
	}
	return []string{providermodels.ModelClaudeFable51}
}

func (c Config) validateAPIMartText() error {
	if !c.FeatureTextClaudeFable51Enabled {
		return nil
	}
	if !c.APIMartProviderEnabled || strings.TrimSpace(c.APIMartAPIKey) == "" || !c.APIMartTextLimitsVerified {
		return fmt.Errorf("config: APIMart text requires enabled provider, API key and APIMART_TEXT_LIMITS_VERIFIED")
	}
	u, err := url.Parse(c.APIMartBaseURL)
	if err != nil || u.Scheme != "https" || u.Host != "api.apimart.ai" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimRight(u.Path, "/") != "/v1" {
		return fmt.Errorf("config: APIMart text base URL must be https://api.apimart.ai/v1")
	}
	return nil
}
