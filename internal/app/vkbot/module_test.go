package vkbot

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

func TestMenuFeaturesPreviewVideoRoutesInDevelopment(t *testing.T) {
	flags := menuFeatures(config.Config{
		Env:                                "development",
		VKMenuVideoEnabled:                 true,
		VKMenuVideoSora2Enabled:            true,
		VKMenuVideoSora2StartEnabled:       true,
		VKMenuVideoSora2ExamplesEnabled:    true,
		VKMenuVideoKling21Enabled:          true,
		VKMenuVideoKling21StartEnabled:     true,
		VKMenuVideoKling21ExamplesEnabled:  true,
		VKMenuVideoSeedance1Enabled:        true,
		VKMenuVideoSeedance1LiteEnabled:    true,
		VKMenuVideoHailuo02Enabled:         true,
		VKMenuVideoHailuo02StandardEnabled: true,
		VKMenuVideoHailuo02FastEnabled:     true,
		VKMenuVideoRoutesPreviewEnabled:    true,
	})

	for _, command := range []domain.CommandType{
		domain.CommandMenuVideoSora2,
		domain.CommandMenuVideoSora2Start,
		domain.CommandMenuVideoSora2Examples,
		domain.CommandMenuVideoKling21Start,
		domain.CommandMenuVideoSeedance1Lite,
		domain.CommandMenuVideoHailuo02,
		domain.CommandMenuVideoHailuo02Standard,
		domain.CommandMenuVideoHailuo02Fast,
	} {
		if !flags.EnabledCommands[command] || flags.DisabledCommands[command] {
			t.Fatalf("command %s must be visible in development preview: enabled=%v disabled=%v", command, flags.EnabledCommands[command], flags.DisabledCommands[command])
		}
	}
}

func TestMenuFeaturesDoNotPreviewVideoRoutesInProduction(t *testing.T) {
	flags := menuFeatures(config.Config{
		Env:                             "production",
		VKMenuVideoEnabled:              true,
		VKMenuVideoSora2Enabled:         true,
		VKMenuVideoSora2StartEnabled:    true,
		VKMenuVideoRoutesPreviewEnabled: true,
	})

	if flags.EnabledCommands[domain.CommandMenuVideoSora2Start] || !flags.DisabledCommands[domain.CommandMenuVideoSora2Start] {
		t.Fatalf("production must not preview disabled video route buttons: enabled=%v disabled=%v", flags.EnabledCommands[domain.CommandMenuVideoSora2Start], flags.DisabledCommands[domain.CommandMenuVideoSora2Start])
	}
}
