package commandrouter

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestRouterParse(t *testing.T) {
	r := New()

	tests := []struct {
		name      string
		input     string
		wantType  domain.CommandType
		wantOp    domain.OperationType
		wantMod   domain.Modality
		wantPromp string
		wantArg   string
	}{
		{
			name:      "image command with prompt",
			input:     "/image neon city at night",
			wantType:  domain.CommandImageGenerate,
			wantOp:    domain.OperationImageGenerate,
			wantMod:   domain.ModalityImage,
			wantPromp: "neon city at night",
		},
		{
			name:      "video command case insensitive",
			input:     "/VIDEO girl walking in Tokyo",
			wantType:  domain.CommandVideoGenerate,
			wantOp:    domain.OperationVideoGenerate,
			wantMod:   domain.ModalityVideo,
			wantPromp: "girl walking in Tokyo",
		},
		{
			name:      "edit command",
			input:     "/edit make the sky purple",
			wantType:  domain.CommandImageEdit,
			wantOp:    domain.OperationImageEdit,
			wantMod:   domain.ModalityImage,
			wantPromp: "make the sky purple",
		},
		{
			name:     "balance control command",
			input:    "/balance",
			wantType: domain.CommandBalance,
		},
		{
			name:     "status with job id arg",
			input:    "/status 1c2d",
			wantType: domain.CommandStatus,
			wantArg:  "1c2d",
		},
		{
			name:     "cancel with job id arg",
			input:    "/cancel 99",
			wantType: domain.CommandCancel,
			wantArg:  "99",
		},
		{
			name:     "help command",
			input:    "  /help  ",
			wantType: domain.CommandHelp,
		},
		{
			name:     "start menu command",
			input:    "/start",
			wantType: domain.CommandStart,
		},
		{
			name:     "start menu command with referral code",
			input:    "/start ABC23456",
			wantType: domain.CommandStart,
			wantArg:  "ABC23456",
		},
		{
			name:     "vk start button",
			input:    "Старт",
			wantType: domain.CommandStart,
		},
		{
			name:     "vk show menu button",
			input:    "Показать меню",
			wantType: domain.CommandShowMenu,
		},
		{
			name:     "vk missing menu repair phrase",
			input:    "нет меню",
			wantType: domain.CommandShowMenu,
		},
		{
			name:     "vk video menu button",
			input:    "🎬 Создать видео",
			wantType: domain.CommandMenuVideo,
		},
		{
			name:     "vk video prunaai button",
			input:    "PrunaAI",
			wantType: domain.CommandMenuVideoPrunaAI,
		},
		{
			name:     "vk video pruna button",
			input:    "Pruna",
			wantType: domain.CommandMenuVideoPrunaAI,
		},
		{
			name:     "vk video hailuo v23 group button",
			input:    "Hailuo v2.3",
			wantType: domain.CommandMenuVideoHailuo02,
		},
		{
			name:     "vk video hailuo v23 standard button",
			input:    "Hailuo v2.3 обычный",
			wantType: domain.CommandMenuVideoHailuo02Standard,
		},
		{
			name:     "vk video hailuo v23 fast button",
			input:    "Hailuo v2.3 Fast",
			wantType: domain.CommandMenuVideoHailuo02Fast,
		},
		{
			name:     "vk video kling v3 button",
			input:    "Kling v3",
			wantType: domain.CommandMenuVideoKling21Start,
		},
		{
			name:     "vk video seedance v2 fast button",
			input:    "Seedance v2 Fast",
			wantType: domain.CommandMenuVideoSeedance1Lite,
		},
		{
			name:     "vk video runway group button",
			input:    "Runway",
			wantType: domain.CommandMenuVideoSora2,
		},
		{
			name:     "vk video runway 45 button",
			input:    "Runway 4.5",
			wantType: domain.CommandMenuVideoSora2Examples,
		},
		{
			name:     "vk video runway 4 turbo button",
			input:    "Runway 4 Turbo",
			wantType: domain.CommandMenuVideoSora2Start,
		},
		{
			name:     "vk video sora model button",
			input:    "Sora 2 — видео текст+фото",
			wantType: domain.CommandMenuVideoSora2,
		},
		{
			name:     "vk video creative product mode",
			input:    "Creative video",
			wantType: domain.CommandMenuVideoSora2Start,
		},
		{
			name:     "vk video balanced product mode",
			input:    "Balanced video",
			wantType: domain.CommandMenuVideoKling21Start,
		},
		{
			name:     "vk video reference product mode",
			input:    "Reference video",
			wantType: domain.CommandMenuVideoSeedance1Lite,
		},
		{
			name:     "vk video cinematic product mode",
			input:    "Cinematic video",
			wantType: domain.CommandMenuVideoHailuo02Standard,
		},
		{
			name:     "vk video fast photo motion product mode",
			input:    "Fast photo motion",
			wantType: domain.CommandMenuVideoHailuo02Fast,
		},
		{
			name:     "vk video seedance lite button",
			input:    "Seedance 1 Lite",
			wantType: domain.CommandMenuVideoSeedance1Lite,
		},
		{
			name:     "vk video seedance pro button",
			input:    "Seedance 1 Pro",
			wantType: domain.CommandMenuVideoSeedance1Pro,
		},
		{
			name:     "vk video hailuo standard button",
			input:    "Hailuo v0.2 Обычный",
			wantType: domain.CommandMenuVideoHailuo02Standard,
		},
		{
			name:     "vk video hailuo fast button",
			input:    "Hailuo v0.2 Fast",
			wantType: domain.CommandMenuVideoHailuo02Fast,
		},
		{
			name:     "vk video back button",
			input:    "⬅️ Назад",
			wantType: domain.CommandShowMenu,
		},
		{
			name:     "vk account menu button",
			input:    "👤 Мой аккаунт",
			wantType: domain.CommandAccount,
		},
		{
			name:     "vk photo text mode button",
			input:    "▶️ Фото по тексту",
			wantType: domain.CommandMenuImageText,
		},
		{
			name:     "vk photo reference mode button",
			input:    "📸 Фото с референсом",
			wantType: domain.CommandMenuImageReference,
		},
		{
			name:     "vk neurohub text menu button",
			input:    "💬 Спросить у НейроХаб",
			wantType: domain.CommandMenuText,
		},
		{
			name:     "vk student solver button",
			input:    "Решальник задач",
			wantType: domain.CommandMenuStudentSolver,
		},
		{
			name:     "vk student presentations button",
			input:    "Генерация презентаций (скоро)",
			wantType: domain.CommandMenuStudentPresentation,
		},
		{
			name:     "vk student reports button",
			input:    "Создание рефератов (скоро)",
			wantType: domain.CommandMenuStudentReport,
		},
		{
			name:     "vk student qa button",
			input:    "❓ Ответы на вопросы",
			wantType: domain.CommandMenuStudentQA,
		},
		{
			name:      "plain text becomes text generate",
			input:     "напиши пост для VK",
			wantType:  domain.CommandTextAsk,
			wantOp:    domain.OperationTextGenerate,
			wantMod:   domain.ModalityText,
			wantPromp: "напиши пост для VK",
		},
		{
			name:      "unknown slash command falls back to text generate",
			input:     "/foo bar",
			wantType:  domain.CommandTextAsk,
			wantOp:    domain.OperationTextGenerate,
			wantMod:   domain.ModalityText,
			wantPromp: "/foo bar",
		},
		{
			name:      "image command without prompt",
			input:     "/image",
			wantType:  domain.CommandImageGenerate,
			wantOp:    domain.OperationImageGenerate,
			wantMod:   domain.ModalityImage,
			wantPromp: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Parse(tt.input)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Operation != tt.wantOp {
				t.Errorf("Operation = %q, want %q", got.Operation, tt.wantOp)
			}
			if got.Modality != tt.wantMod {
				t.Errorf("Modality = %q, want %q", got.Modality, tt.wantMod)
			}
			if got.Prompt != tt.wantPromp {
				t.Errorf("Prompt = %q, want %q", got.Prompt, tt.wantPromp)
			}
			if got.Arg != tt.wantArg {
				t.Errorf("Arg = %q, want %q", got.Arg, tt.wantArg)
			}
		})
	}
}

func TestResultCreatesJob(t *testing.T) {
	r := New()

	jobCommands := []string{"/image cat", "/video dog", "/edit fix", "just text"}
	for _, in := range jobCommands {
		if !r.Parse(in).CreatesJob() {
			t.Errorf("expected %q to create a job", in)
		}
	}

	controlCommands := []string{"/balance", "/status 1", "/cancel 1", "/help", "/start", "Старт", "Показать меню", "нет меню", "🎬 Создать видео", "Pruna", "Hailuo v2.3", "Hailuo v2.3 Обычный", "Hailuo v2.3 Fast", "Kling v3", "Seedance v2 Fast", "Runway", "Runway 4.5", "Runway 4 Turbo", "Creative video", "Balanced video", "Reference video", "Cinematic video", "Fast photo motion", "Sora 2 — видео текст+фото", "Seedance 1 Lite", "Seedance 1 Pro", "Hailuo v0.2 Обычный", "Hailuo v0.2 Fast", "⬅️ Назад", "👤 Мой аккаунт", "▶️ Фото по тексту", "📸 Фото с референсом", "💬 Спросить у НейроХаб", "💬 Спросить у GPT", "Решальник задач", "Генерация презентаций (скоро)", "Создание рефератов (скоро)", "❓ Ответы на вопросы"}
	for _, in := range controlCommands {
		if r.Parse(in).CreatesJob() {
			t.Errorf("expected %q to NOT create a job", in)
		}
	}
}
