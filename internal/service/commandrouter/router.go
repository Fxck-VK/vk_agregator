// Package commandrouter turns a raw VK message into a normalized domain
// command. It is intentionally free of any AI-provider, billing or VK-delivery
// knowledge: its only job is to classify intent and extract a prompt/arguments
// so that downstream services can create a Job.
package commandrouter

import (
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// Result is the normalized outcome of parsing a single message.
type Result struct {
	// Type is the classified command intent.
	Type domain.CommandType
	// Operation is the AI operation to run when the command creates a job. It is
	// the empty string for control commands (balance, status, cancel, help).
	Operation domain.OperationType
	// Modality is the content kind of the operation, empty for control commands.
	Modality domain.Modality
	// Prompt is the user text with any leading command token stripped.
	Prompt string
	// Arg is the single positional argument for control commands that take one
	// (for example the job id passed to /status or /cancel). Empty otherwise.
	Arg string
}

// CreatesJob reports whether the parsed command should produce a billable Job.
func (r Result) CreatesJob() bool {
	return r.Type.CreatesJob()
}

// Router classifies messages into commands. It holds no state and is safe for
// concurrent use.
type Router struct{}

// New returns a ready-to-use Router.
func New() *Router {
	return &Router{}
}

// Parse classifies a raw message. Recognized slash commands map to their
// command type; any other text (including unrecognized slash input) is treated
// as a free-form text generation request.
func (r *Router) Parse(rawText string) Result {
	trimmed := strings.TrimSpace(rawText)
	token, rest := splitFirstToken(trimmed)

	switch strings.ToLower(token) {
	case "/image":
		return Result{
			Type:      domain.CommandImageGenerate,
			Operation: domain.OperationImageGenerate,
			Modality:  domain.ModalityImage,
			Prompt:    rest,
		}
	case "/video":
		return Result{
			Type:      domain.CommandVideoGenerate,
			Operation: domain.OperationVideoGenerate,
			Modality:  domain.ModalityVideo,
			Prompt:    rest,
		}
	case "/edit":
		return Result{
			Type:      domain.CommandImageEdit,
			Operation: domain.OperationImageEdit,
			Modality:  domain.ModalityImage,
			Prompt:    rest,
		}
	case "/balance":
		return Result{Type: domain.CommandBalance}
	case "/status":
		return Result{Type: domain.CommandStatus, Arg: strings.TrimSpace(rest)}
	case "/cancel":
		return Result{Type: domain.CommandCancel, Arg: strings.TrimSpace(rest)}
	case "/help":
		return Result{Type: domain.CommandHelp}
	default:
		// Anything else, including plain text and unknown slash input, becomes a
		// text generation request carrying the full original message.
		return Result{
			Type:      domain.CommandTextAsk,
			Operation: domain.OperationTextGenerate,
			Modality:  domain.ModalityText,
			Prompt:    trimmed,
		}
	}
}

// splitFirstToken splits s into its first whitespace-delimited token and the
// remaining text (with surrounding whitespace trimmed).
func splitFirstToken(s string) (token, rest string) {
	if s == "" {
		return "", ""
	}
	if idx := strings.IndexFunc(s, isSpace); idx >= 0 {
		return s[:idx], strings.TrimSpace(s[idx+1:])
	}
	return s, ""
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
