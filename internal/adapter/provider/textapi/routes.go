package textapi

import "vk-ai-aggregator/internal/service/providermodels"

type protocol uint8

const (
	responses protocol = iota
	messages
	geminiChat
	openAIChat
)

type modelRoute struct {
	path    string
	format  protocol
	credits int64
}

// Exact upstream contracts; similar names never imply interchangeable routes.
func route(model string) (modelRoute, bool) {
	switch model {
	case providermodels.ModelGPT55:
		return modelRoute{"/codex/v1/responses", responses, 6}, true
	case providermodels.ModelGPT56Terra:
		return modelRoute{"/codex/v1/responses", responses, 3}, true
	case providermodels.ModelGPT6Astra:
		return modelRoute{"/codex/v1/responses", responses, 11}, true
	case providermodels.ModelClaudeOpus47:
		return modelRoute{"/claude/v1/messages", messages, 6}, true
	case providermodels.ModelClaudeOpus48, providermodels.ModelClaudeOpus5:
		return modelRoute{"/claude/v1/messages", messages, 8}, true
	case providermodels.ModelClaudeFable5:
		return modelRoute{"/claude/v1/messages", messages, 15}, true
	case providermodels.ModelGemini31Pro:
		return modelRoute{"/gemini-3.1-pro/v1/chat/completions", geminiChat, 3}, true
	case providermodels.ModelGemini37Flash:
		return modelRoute{"/gemini-3-7-flash-openai/v1/chat/completions", geminiChat, 1}, true
	case providermodels.ModelGemini36Flash:
		return modelRoute{"/gemini-3-6-flash-openai/v1/chat/completions", geminiChat, 1}, true
	case providermodels.ModelClaudeFable51:
		return modelRoute{"/chat/completions", openAIChat, 2}, true
	}
	return modelRoute{}, false
}
