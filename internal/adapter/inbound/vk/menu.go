package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
)

type menuScreen struct {
	text                     func(balance int64) string
	keyboard                 func() *vkdelivery.Keyboard
	includeWelcomeAttachment bool
	needsBalance             bool
}

var menuScreens = map[domain.CommandType]menuScreen{
	domain.CommandStart: {
		text:                     welcomeText,
		keyboard:                 welcomeKeyboard,
		includeWelcomeAttachment: true,
		needsBalance:             true,
	},
	domain.CommandShowMenu: {
		text:                     welcomeText,
		keyboard:                 welcomeKeyboard,
		includeWelcomeAttachment: true,
		needsBalance:             true,
	},
	domain.CommandHelp: {
		text:                     welcomeText,
		keyboard:                 welcomeKeyboard,
		includeWelcomeAttachment: true,
		needsBalance:             true,
	},
	domain.CommandBalance: {
		text:         accountText,
		keyboard:     accountKeyboard,
		needsBalance: true,
	},
	domain.CommandAccount: {
		text:         accountText,
		keyboard:     accountKeyboard,
		needsBalance: true,
	},
	domain.CommandTopUp: {
		text:     fixedText(topUpText),
		keyboard: backKeyboard,
	},
	domain.CommandMenuText: {
		text:     fixedText(gptActiveText),
		keyboard: backKeyboard,
	},
	domain.CommandMenuImage: {
		text:     fixedText(photoIntroText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageText: {
		text:     fixedText(photoTextModeText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageReference: {
		text:     fixedText(photoReferenceModeText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuVideo: {
		text:     fixedText("Выбери модель для генерации:"),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoSora2: {
		text:     fixedText("Sora 2 выбрана.\n\nОтправьте описание видео командой /video. Выбор конкретной модели будет подключен в следующем шаге."),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoKling21: {
		text:     fixedText("Kling v2.1 выбрана.\n\nОтправьте описание видео командой /video. Выбор конкретной модели будет подключен в следующем шаге."),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoSeedance1: {
		text:     fixedText("Seedance 1 выбрана.\n\nОтправьте описание видео командой /video. Выбор конкретной модели будет подключен в следующем шаге."),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoHaiuo02: {
		text:     fixedText("Haiuo v0.2 выбрана.\n\nОтправьте описание видео командой /video. Выбор конкретной модели будет подключен в следующем шаге."),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuStudents: {
		text:     fixedText(studentsText),
		keyboard: studentsKeyboard,
	},
	domain.CommandMenuStudentSolver: {
		text:     fixedText("Решальник задач активен.\n\nПришлите условие задачи обычным сообщением, и GPT поможет разобрать решение пошагово."),
		keyboard: studentsKeyboard,
	},
	domain.CommandMenuStudentPresentation: {
		text:     fixedText("Генерация презентаций скоро появится.\n\nПока можно попросить GPT составить структуру презентации обычным сообщением."),
		keyboard: studentsKeyboard,
	},
	domain.CommandMenuStudentReport: {
		text:     fixedText("Создание рефератов скоро появится.\n\nПока можно попросить GPT составить план, тезисы или черновик обычным сообщением."),
		keyboard: studentsKeyboard,
	},
	domain.CommandMenuStudentQA: {
		text:     fixedText("Ответы на вопросы активны.\n\nНапишите учебный вопрос обычным сообщением, и GPT поможет с объяснением."),
		keyboard: studentsKeyboard,
	},
}

const (
	gptActiveText = "🤖 SUPER GPT активен!\n\nЯ готов ответить на любые вопросы и помочь с идеями\nСпроси что-нибудь прямо сейчас!"

	photoIntroText = "✅ У вас есть 1 бесплатная попытка в сутки на генерацию с текстом.\n\n▶️ Генерация фото по тексту – это когда ты пишешь, что хочешь увидеть (например, \"кот в очках на пляже\"), а ИИ сам «придумывает» и рисует такую картинку. (1 бесплатно)\n\n📸Генерация фото по тексту и фото (с референсом) – ИИ использует твою фотографию как образец — он сохраняет стиль, позу, цвета, но уже с новым содержанием по твоему описанию. (Только платные)"

	photoTextModeText      = "▶️ Генерация фото по тексту выбрана.\n\nОпишите, что хотите увидеть, командой /image.\n\nПример:\n/image кот в очках на пляже"
	photoReferenceModeText = "📸 Генерация фото с референсом пока будет подключена после входящих фото-артефактов.\n\nСейчас доступна генерация по тексту через /image."

	studentsText = "🎁Данные нейронные сети помогут вам во время учебы"

	topUpText = "💰 Пополнить баланс\n\nПополнение будет подключено отдельным платежным потоком. Пока для тестирования доступны стартовые кредиты."
)

func controlTypeFromPayload(payload string) (domain.CommandType, bool) {
	if payload == "" {
		return "", false
	}
	var data struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return "", false
	}
	t := domain.CommandType(data.Command)
	if !isMenuCommand(t) {
		return "", false
	}
	return t, true
}

func shouldSendControlResponse(t domain.CommandType) bool {
	return isMenuCommand(t)
}

func isMenuCommand(t domain.CommandType) bool {
	_, ok := menuScreens[t]
	return ok
}

func fixedText(text string) func(int64) string {
	return func(int64) string {
		return text
	}
}

func (h *Handler) sendControlResponse(ctx context.Context, t domain.CommandType, idemKey string, peerID int64, user *domain.User) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk control response skipped because VK_ACCESS_TOKEN is not configured",
			slog.String("command_type", string(t)))
		return nil
	}

	if t == domain.CommandStart {
		if err := h.sendPersistentMenuButton(ctx, idemKey, peerID); err != nil {
			return fmt.Errorf("send persistent menu button: %w", err)
		}
	}

	screen, ok := menuScreens[t]
	if !ok {
		screen = menuScreens[domain.CommandShowMenu]
	}

	balance := int64(0)
	if screen.needsBalance {
		acc, err := h.deps.Billing.EnsureAccount(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("ensure billing account: %w", err)
		}
		balance = acc.BalanceCached
	}

	msg := vkdelivery.Message{
		Text:     screen.text(balance),
		Keyboard: screen.keyboard(),
	}
	if screen.includeWelcomeAttachment {
		msg.Attachment = h.cfg.WelcomeAttachment
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control:" + idemKey + ":" + string(t))
	_, err := h.deps.Control.SendMessage(ctx, peerID, randomID, msg)
	if err == nil {
		return nil
	}
	if msg.Keyboard == nil || !vkdelivery.IsAPIErrorCode(err, 911, 912) {
		return err
	}

	h.logger.Warn("vk keyboard send failed; retrying control response without keyboard",
		slog.String("command_type", string(t)),
		slog.String("error", err.Error()))
	msg.Keyboard = nil
	_, err = h.deps.Control.SendMessage(ctx, peerID, randomID, msg)
	return err
}

func (h *Handler) sendPersistentMenuButton(ctx context.Context, idemKey string, peerID int64) error {
	msg := vkdelivery.Message{
		Text:     "Меню теперь доступно по кнопке «Показать меню».",
		Keyboard: menuAccessKeyboard(),
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_keyboard:" + idemKey)
	_, err := h.deps.Control.SendMessage(ctx, peerID, randomID, msg)
	if err == nil {
		return nil
	}
	if vkdelivery.IsAPIErrorCode(err, 911, 912) {
		h.logger.Warn("vk persistent keyboard send skipped",
			slog.String("error", err.Error()))
		return nil
	}
	return err
}

func welcomeText(balance int64) string {
	return fmt.Sprintf("👋 Добро пожаловать в Super GPT!\n\n🤖 Здесь вы можете создавать уникальные тексты, генерировать изображения и видео с помощью нейросетей!\n\nВаш баланс: %d 💎\n🎁 Вам доступна одна бесплатная генерация\n\n📌 Совет: Закрепляй бота и используй промты, которые мы оставили в каждой генеративной модели.", balance)
}

func accountText(balance int64) string {
	return fmt.Sprintf("👤 Мой аккаунт\n\nВаш баланс: %d 💎\n\nВыберите действие:", balance)
}

func welcomeKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("🎬 Создать видео", domain.CommandMenuVideo, "primary"),
			},
			{
				button("🖼️ Создать фото", domain.CommandMenuImage, "primary"),
			},
			{
				button("💬 Спросить у GPT", domain.CommandMenuText, "secondary"),
			},
			{
				button("🎁 Студентам и школьникам", domain.CommandMenuStudents, "secondary"),
			},
			{
				button("👤 Мой аккаунт", domain.CommandAccount, "secondary"),
				button("💰 Пополнить баланс", domain.CommandTopUp, "positive"),
			},
		},
	}
}

func videoModelKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Sora 2 — видео текст+фото", domain.CommandMenuVideoSora2, "secondary"),
			},
			{
				button("Kling v2.1 — видео текст+фото", domain.CommandMenuVideoKling21, "secondary"),
			},
			{
				button("Seedance 1 — видео по тексту", domain.CommandMenuVideoSeedance1, "secondary"),
			},
			{
				button("Haiuo v0.2 — видео текст+фото", domain.CommandMenuVideoHaiuo02, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func photoModeKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("▶️ Фото по тексту", domain.CommandMenuImageText, "primary"),
			},
			{
				button("📸 Фото с референсом", domain.CommandMenuImageReference, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func studentsKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Решальник задач", domain.CommandMenuStudentSolver, "secondary"),
			},
			{
				button("Генерация презентаций (скоро)", domain.CommandMenuStudentPresentation, "secondary"),
			},
			{
				button("Создание рефератов (скоро)", domain.CommandMenuStudentReport, "secondary"),
			},
			{
				button("❓ Ответы на вопросы", domain.CommandMenuStudentQA, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func backKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func accountKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("💰 Пополнить баланс", domain.CommandTopUp, "positive"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func menuAccessKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  false,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Показать меню", domain.CommandShowMenu, "primary"),
			},
		},
	}
}

func button(label string, command domain.CommandType, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(struct {
		Command string `json:"command"`
	}{Command: string(command)})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}
