// Package kie binds the shared synchronous text adapter to KIE credentials and routes.
package kie

import (
	"vk-ai-aggregator/internal/adapter/provider/textapi"
	"vk-ai-aggregator/internal/domain"
)

type Config = textapi.Config
type Provider = textapi.Provider
type Error = textapi.Error

func New(cfg Config) *Provider {
	cfg.Provider = domain.ProviderKIE
	return textapi.New(cfg)
}
