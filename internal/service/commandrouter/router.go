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
	normalized := strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
	switch normalized {
	case "/start", "start", "старт", "🚀 старт", "▶️ старт", "начать":
		return Result{Type: domain.CommandStart}
	case "показать меню", "меню", "нет меню", "нет кнопки", "где меню", "menu":
		return Result{Type: domain.CommandShowMenu}
	case "🎬 создать видео", "создать видео":
		return Result{Type: domain.CommandMenuVideo}
	case "prunaai", "pruna ai":
		return Result{Type: domain.CommandMenuVideoPrunaAI}
	case "runway gen-4 turbo", "runway gen4 turbo", "creative video", "sora 2 начать генерацию":
		return Result{Type: domain.CommandMenuVideoSora2Start}
	case "sora 2 — видео текст+фото", "sora 2 - видео текст+фото", "sora 2 examples", "sora 2 примеры":
		return Result{Type: domain.CommandMenuVideoSora2}
	case "kling o3 standard", "balanced video", "kling v2.1 начать генерацию":
		return Result{Type: domain.CommandMenuVideoKling21Start}
	case "kling v2.1 — видео текст+фото", "kling v2.1 - видео текст+фото", "kling v2.1 examples", "kling v2.1 примеры":
		return Result{Type: domain.CommandMenuVideoKling21}
	case "seedance 2.0 fast", "seedance 2 fast", "reference video", "seedance 1 lite":
		return Result{Type: domain.CommandMenuVideoSeedance1Lite}
	case "seedance 1 — видео по тексту", "seedance 1 - видео по тексту":
		return Result{Type: domain.CommandMenuVideoSeedance1}
	case "seedance 2.0 pro", "seedance 2 pro", "seedance 1 pro":
		return Result{Type: domain.CommandMenuVideoSeedance1Pro}
	case "hailuo 2.3 fast", "fast photo motion", "hailuo v0.2 fast", "haiuo v0.2 fast":
		return Result{Type: domain.CommandMenuVideoHailuo02Fast}
	case "hailuo v0.2 — видео текст+фото", "hailuo v0.2 - видео текст+фото",
		"haiuo v0.2 — видео текст+фото", "haiuo v0.2 - видео текст+фото":
		return Result{Type: domain.CommandMenuVideoHailuo02}
	case "hailuo 2.3 standard", "cinematic video", "hailuo v0.2 обычный", "hailuo v0.2 standard", "haiuo v0.2 обычный", "haiuo v0.2 standard":
		return Result{Type: domain.CommandMenuVideoHailuo02Standard}
	case "⬅️ назад", "назад":
		return Result{Type: domain.CommandShowMenu}
	case "🖼️ создать фото", "🖼 создать фото", "создать фото", "создать изображение":
		return Result{Type: domain.CommandMenuImage}
	case "▶️ фото по тексту", "▶ фото по тексту", "фото по тексту":
		return Result{Type: domain.CommandMenuImageText}
	case "nano banana 2", "nano banana 2 image":
		return Result{Type: domain.CommandMenuImageNanoBanana2}
	case "nano banana pro", "nano banana pro image":
		return Result{Type: domain.CommandMenuImageText}
	case "seedream 4.5", "seedream", "deepinfra seedream", "deepinfra seedream 4.5", "bytedance seedream 4.5", "bytedance/seedream-4.5":
		return Result{Type: domain.CommandMenuImageDeepInfraSeedream}
	case "sdxl turbo", "sdxl", "deepinfra sdxl", "deepinfra sdxl turbo", "nano banana flash", "stability ai sdxl turbo", "stabilityai/sdxl-turbo":
		return Result{Type: domain.CommandMenuImageDeepInfraSDXL}
	case "gpt image 2", "gpt-image-2", "gpt_image_2":
		return Result{Type: domain.CommandMenuImageGPTImage2}
	case "1k", "1k quality", "качество 1k":
		return Result{Type: domain.CommandMenuImageQuality1K}
	case "2k", "2k quality", "качество 2k":
		return Result{Type: domain.CommandMenuImageQuality2K}
	case "4k", "4k quality", "качество 4k":
		return Result{Type: domain.CommandMenuImageQuality4K}
	case "назад к качеству", "back to quality":
		return Result{Type: domain.CommandMenuImageBackToQuality}
	case "📸 фото с референсом", "фото с референсом", "фото по тексту и фото":
		return Result{Type: domain.CommandMenuImageReference}
	case "💬 спросить у нейрохаб", "спросить у нейрохаб", "💬 спросить у gpt", "спросить у gpt", "задать вопрос":
		return Result{Type: domain.CommandMenuText}
	case "🎁 студентам и школьникам", "студентам и школьникам":
		return Result{Type: domain.CommandMenuStudents}
	case "решальник задач":
		return Result{Type: domain.CommandMenuStudentSolver}
	case "генерация презентаций (скоро)", "генерация презентаций":
		return Result{Type: domain.CommandMenuStudentPresentation}
	case "создание рефератов (скоро)", "создание рефератов":
		return Result{Type: domain.CommandMenuStudentReport}
	case "❓ ответы на вопросы", "ответы на вопросы":
		return Result{Type: domain.CommandMenuStudentQA}
	case "👤 мой аккаунт", "мой аккаунт", "аккаунт":
		return Result{Type: domain.CommandAccount}
	case "💰 пополнить баланс", "пополнить баланс":
		return Result{Type: domain.CommandTopUp}
	}
	token, rest := splitFirstToken(trimmed)
	if isStartToken(strings.ToLower(token)) {
		arg, _ := splitFirstToken(rest)
		return Result{Type: domain.CommandStart, Arg: arg}
	}

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

func isStartToken(token string) bool {
	switch token {
	case "/start", "start", "\u0441\u0442\u0430\u0440\u0442", "\u043d\u0430\u0447\u0430\u0442\u044c", "СЃС‚Р°СЂС‚", "РЅР°С‡Р°С‚СЊ":
		return true
	default:
		return false
	}
}
