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
		text:     fixedText(sora2Text),
		keyboard: sora2Keyboard,
	},
	domain.CommandMenuVideoSora2Start: {
		text:     fixedText(sora2StartText),
		keyboard: sora2BackKeyboard,
	},
	domain.CommandMenuVideoSora2Examples: {
		text:     fixedText(sora2ExamplesText),
		keyboard: sora2BackKeyboard,
	},
	domain.CommandMenuVideoKling21: {
		text:     fixedText(kling21Text),
		keyboard: kling21Keyboard,
	},
	domain.CommandMenuVideoKling21Start: {
		text:     fixedText(kling21StartText),
		keyboard: kling21BackKeyboard,
	},
	domain.CommandMenuVideoKling21Examples: {
		text:     fixedText(kling21ExamplesText),
		keyboard: kling21BackKeyboard,
	},
	domain.CommandMenuVideoSeedance1: {
		text:     fixedText(seedance1Text),
		keyboard: seedance1Keyboard,
	},
	domain.CommandMenuVideoSeedance1Lite: {
		text:     fixedText(seedance1LiteText),
		keyboard: seedance1BackKeyboard,
	},
	domain.CommandMenuVideoSeedance1Pro: {
		text:     fixedText(seedance1ProText),
		keyboard: seedance1BackKeyboard,
	},
	domain.CommandMenuVideoHaiuo02: {
		text:     fixedText(haiuo02Text),
		keyboard: haiuo02Keyboard,
	},
	domain.CommandMenuVideoHaiuo02Standard: {
		text:     fixedText(haiuo02StandardText),
		keyboard: haiuo02BackKeyboard,
	},
	domain.CommandMenuVideoHaiuo02Fast: {
		text:     fixedText(haiuo02FastText),
		keyboard: haiuo02BackKeyboard,
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

	sora2Text         = "sora-2\nОписание: Генерирует видео по тексту или фото.\n\n“ A hyper-realistic police bodycam video of a kangaroo making punching feints toward a police officer on a dusty rural road in Australia. The kangaroo stands ”\n\n🔗Инструкция: https://t.me/sora_video_1"
	sora2StartText    = "sora-2\n\nВвод промпта для этой модели будет подключен следующим шагом.\n\nПока можно вернуться назад и выбрать другой режим."
	sora2ExamplesText = "ℹ️ Примеры sora-2\n\n1. A cinematic drone shot over a neon city at night, rain reflections on the street, realistic camera movement.\n\n2. A close-up handheld video of a chef cutting fruit in a bright kitchen, natural motion, realistic details."

	kling21Text         = "Kling v2.1 master\nОписание: Генерирует видео по тексту или фото.\n\n“ The setting has warm lighting from streetlights or soft party lights. A little boy around 2 to 3 years old, with light skin tone, brown hair, and big green ”\n\n🔗Инструкция: https://t.me/pakrnet"
	kling21StartText    = "Kling v2.1 master\n\nВвод промпта для этой модели будет подключен следующим шагом.\n\nПока можно вернуться назад и выбрать другой режим."
	kling21ExamplesText = "ℹ️ Примеры Kling v2.1\n\n1. A warm cinematic scene of friends walking under streetlights, soft party lights, realistic skin and motion.\n\n2. A product video of a glass bottle rotating on a table, studio lighting, smooth camera movement."

	seedance1Text     = "🔹 Seedance\n\nLite — это как «лайт-версия» приложения: самое простое, чтобы попробовать.\n\nPro — это как «премиум»: больше функций, настроек и возможностей для крутого результата.\n\n☝️ Если хочешь быстро и просто — бери Lite. Если любишь «по максимуму» — тогда Pro."
	seedance1LiteText = "Seedance 1 Lite выбран.\n\nВвод промпта для этого варианта будет подключен следующим шагом."
	seedance1ProText  = "Seedance 1 Pro выбран.\n\nВвод промпта для этого варианта будет подключен следующим шагом."

	haiuo02Text         = "🔹 Haiuo 02\n\nHaiuo 02 — картинка суперчёткая, реалистичная, прям как в фильме.\n\nHaiuo 02 Fast — версия «на скорость»: делает видео быстрее, но качество чуть ниже.\n\n☝️ Если нужен «вау-визуал» — бери Haiuo. Если важнее быстро и удобно — Fast."
	haiuo02StandardText = "Haiuo v0.2 Обычный выбран.\n\nВвод промпта для этого варианта будет подключен следующим шагом."
	haiuo02FastText     = "Haiuo v0.2 Fast выбран.\n\nВвод промпта для этого варианта будет подключен следующим шагом."

	studentsText = "🎁Данные нейронные сети помогут вам во время учебы"

	topUpText = "💰 Пополнить баланс\n\nПополнение будет подключено отдельным платежным потоком. Пока для тестирования доступны стартовые кредиты."

	chooseModeText = "Выберите режим в меню выше."
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

func (h *Handler) sendControlResponse(ctx context.Context, t domain.CommandType, idemKey string, peerID int64, user *domain.User, allowEdit bool) error {
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
	h.filterMenuKeyboard(msg.Keyboard)
	if msg.Keyboard != nil && len(msg.Keyboard.Buttons) == 0 {
		msg.Keyboard = nil
	}
	h.applyMenuButtonMode(msg.Keyboard)
	if screen.includeWelcomeAttachment {
		msg.Attachment = h.cfg.WelcomeAttachment
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control:" + idemKey + ":" + string(t))
	result, err := h.deliverControlResponse(ctx, t, peerID, randomID, msg, allowEdit)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
		return nil
	}
	return err
}

func (h *Handler) deliverControlResponse(ctx context.Context, t domain.CommandType, peerID, randomID int64, msg vkdelivery.Message, allowEdit bool) (vkdelivery.SendResult, error) {
	if allowEdit {
		if active, ok := h.getActiveMenu(peerID); ok {
			result, err := h.editControlMessage(ctx, t, peerID, active.MessageID, msg)
			if err == nil {
				return result, nil
			}
			h.clearActiveMenu(peerID)
			h.logger.Warn("vk menu edit failed; sending a new control response",
				slog.String("command_type", string(t)),
				slog.Int64("peer_id", peerID),
				slog.Int64("message_id", active.MessageID),
				slog.String("error", err.Error()))
		}
	}
	return h.sendControlMessage(ctx, t, peerID, randomID, msg)
}

func (h *Handler) editControlMessage(ctx context.Context, t domain.CommandType, peerID, messageID int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	result, err := h.deps.Control.EditMessage(ctx, peerID, messageID, msg)
	if err == nil {
		return result, nil
	}
	if msg.Keyboard == nil || !vkdelivery.IsAPIErrorCode(err, 911, 912) {
		return vkdelivery.SendResult{}, err
	}

	h.logger.Warn("vk keyboard edit failed; retrying control response edit without keyboard",
		slog.String("command_type", string(t)),
		slog.String("error", err.Error()))
	msg.Keyboard = nil
	return h.deps.Control.EditMessage(ctx, peerID, messageID, msg)
}

func (h *Handler) sendControlMessage(ctx context.Context, t domain.CommandType, peerID, randomID int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	result, err := h.deps.Control.SendMessage(ctx, peerID, randomID, msg)
	if err == nil {
		return result, nil
	}
	if msg.Keyboard == nil || !vkdelivery.IsAPIErrorCode(err, 911, 912) {
		return vkdelivery.SendResult{}, err
	}

	h.logger.Warn("vk keyboard send failed; retrying control response without keyboard",
		slog.String("command_type", string(t)),
		slog.String("error", err.Error()))
	msg.Keyboard = nil
	return h.deps.Control.SendMessage(ctx, peerID, randomID, msg)
}

func (h *Handler) sendUnroutedTextResponse(ctx context.Context, idemKey string, peerID int64) error {
	if h.cfg.UnroutedTextMode != unroutedTextModeReply {
		return nil
	}
	if h.deps.Control == nil {
		h.logger.Warn("vk unrouted text response skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}

	msg := vkdelivery.Message{
		Text: chooseModeText,
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_unrouted:" + idemKey)
	_, err := h.sendControlMessage(ctx, domain.CommandShowMenu, peerID, randomID, msg)
	return err
}

func (h *Handler) filterMenuKeyboard(keyboard *vkdelivery.Keyboard) {
	if keyboard == nil {
		return
	}
	filteredRows := make([][]vkdelivery.KeyboardButton, 0, len(keyboard.Buttons))
	for _, row := range keyboard.Buttons {
		filteredRow := make([]vkdelivery.KeyboardButton, 0, len(row))
		for _, button := range row {
			command, ok := controlTypeFromPayload(button.Payload)
			if ok && !h.menuCommandEnabled(command) {
				continue
			}
			filteredRow = append(filteredRow, button)
		}
		if len(filteredRow) > 0 {
			filteredRows = append(filteredRows, filteredRow)
		}
	}
	keyboard.Buttons = filteredRows
}

func (h *Handler) menuCommandEnabled(command domain.CommandType) bool {
	if h.cfg.MenuFeatures.DisabledCommands[command] {
		return false
	}
	switch command {
	case domain.CommandMenuVideoSora2,
		domain.CommandMenuVideoSora2Start,
		domain.CommandMenuVideoSora2Examples,
		domain.CommandMenuVideoKling21,
		domain.CommandMenuVideoKling21Start,
		domain.CommandMenuVideoKling21Examples,
		domain.CommandMenuVideoSeedance1,
		domain.CommandMenuVideoSeedance1Lite,
		domain.CommandMenuVideoSeedance1Pro,
		domain.CommandMenuVideoHaiuo02,
		domain.CommandMenuVideoHaiuo02Standard,
		domain.CommandMenuVideoHaiuo02Fast:
		return h.menuCommandEnabled(domain.CommandMenuVideo)
	case domain.CommandMenuImageText,
		domain.CommandMenuImageReference:
		return h.menuCommandEnabled(domain.CommandMenuImage)
	case domain.CommandMenuStudentSolver,
		domain.CommandMenuStudentPresentation,
		domain.CommandMenuStudentReport,
		domain.CommandMenuStudentQA:
		return h.menuCommandEnabled(domain.CommandMenuStudents)
	default:
		return true
	}
}

func (h *Handler) getActiveMenu(peerID int64) (activeMenuMessage, bool) {
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	msg, ok := h.activeMenus[peerID]
	return msg, ok
}

func (h *Handler) setActiveMenu(peerID, messageID int64) {
	if messageID == 0 {
		return
	}
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	h.activeMenus[peerID] = activeMenuMessage{MessageID: messageID}
}

func (h *Handler) clearActiveMenu(peerID int64) {
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	delete(h.activeMenus, peerID)
}

func (h *Handler) applyMenuButtonMode(keyboard *vkdelivery.Keyboard) {
	if keyboard == nil || !keyboard.Inline {
		return
	}
	actionType := "text"
	if h.cfg.MenuButtonMode == "callback" {
		actionType = "callback"
	}
	for row := range keyboard.Buttons {
		for col := range keyboard.Buttons[row] {
			keyboard.Buttons[row][col].ActionType = actionType
		}
	}
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

func sora2Keyboard() *vkdelivery.Keyboard {
	return videoDetailKeyboard(domain.CommandMenuVideoSora2Start, domain.CommandMenuVideoSora2Examples)
}

func sora2BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoSora2)
}

func kling21Keyboard() *vkdelivery.Keyboard {
	return videoDetailKeyboard(domain.CommandMenuVideoKling21Start, domain.CommandMenuVideoKling21Examples)
}

func kling21BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoKling21)
}

func videoDetailKeyboard(startCommand, examplesCommand domain.CommandType) *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("😀 Начать генерацию", startCommand, "primary"),
			},
			{
				button("ℹ️ Примеры", examplesCommand, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandMenuVideo, "secondary"),
			},
		},
	}
}

func seedance1Keyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Seedance 1 Lite", domain.CommandMenuVideoSeedance1Lite, "secondary"),
			},
			{
				button("Seedance 1 Pro", domain.CommandMenuVideoSeedance1Pro, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandMenuVideo, "secondary"),
			},
		},
	}
}

func seedance1BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoSeedance1)
}

func haiuo02Keyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Haiuo v0.2 Обычный", domain.CommandMenuVideoHaiuo02Standard, "secondary"),
			},
			{
				button("Haiuo v0.2 Fast", domain.CommandMenuVideoHaiuo02Fast, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandMenuVideo, "secondary"),
			},
		},
	}
}

func haiuo02BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoHaiuo02)
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

func backToKeyboard(command domain.CommandType) *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("⬅️ Назад", command, "secondary"),
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
