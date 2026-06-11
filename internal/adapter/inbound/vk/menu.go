package vk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

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
		keyboard:     emptyAccountKeyboard,
		needsBalance: true,
	},
	domain.CommandAccount: {
		text:         accountText,
		keyboard:     emptyAccountKeyboard,
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
		text:     fixedText(photoTextPromptInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageText: {
		text:     fixedText(photoTextModeInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageReference: {
		text:     fixedText(photoReferenceModeInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuVideo: {
		text:     fixedText("Выбери модель для генерации:"),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoPrunaAI: {
		text:     fixedText(prunaAIText),
		keyboard: prunaAIBackKeyboard,
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
	gptActiveText = "🤖 НейроХаб активен!\n\nЯ готов ответить на любые вопросы и помочь с идеями\nСпроси что-нибудь прямо сейчас!"

	prunaAIText = "PrunaAI активен.\n\nНапишите описание видео обычным сообщением.\n\nПример: кот в очках едет на жирафе."

	sora2Text         = "sora-2\nОписание: Генерирует видео по тексту или фото.\n\n“ A hyper-realistic police bodycam video of a kangaroo making punching feints toward a police officer on a dusty rural road in Australia. The kangaroo stands ”\n\n🔗Инструкция: https://t.me/sora_video_1"
	sora2StartText    = "sora-2 активен.\n\nНапишите описание видео обычным сообщением.\n\nПример: cinematic drone shot over a neon city at night, rain reflections, realistic camera movement."
	sora2ExamplesText = "ℹ️ Примеры sora-2\n\n1. A cinematic drone shot over a neon city at night, rain reflections on the street, realistic camera movement.\n\n2. A close-up handheld video of a chef cutting fruit in a bright kitchen, natural motion, realistic details."

	kling21Text         = "Kling v2.1 master\nОписание: Генерирует видео по тексту или фото.\n\n“ The setting has warm lighting from streetlights or soft party lights. A little boy around 2 to 3 years old, with light skin tone, brown hair, and big green ”\n\n🔗Инструкция: https://t.me/pakrnet"
	kling21StartText    = "Kling v2.1 master активен.\n\nНапишите описание видео обычным сообщением.\n\nПример: warm cinematic scene, friends walking under streetlights, realistic motion."
	kling21ExamplesText = "ℹ️ Примеры Kling v2.1\n\n1. A warm cinematic scene of friends walking under streetlights, soft party lights, realistic skin and motion.\n\n2. A product video of a glass bottle rotating on a table, studio lighting, smooth camera movement."

	seedance1Text     = "🔹 Seedance\n\nLite — это как «лайт-версия» приложения: самое простое, чтобы попробовать.\n\nPro — это как «премиум»: больше функций, настроек и возможностей для крутого результата.\n\n☝️ Если хочешь быстро и просто — бери Lite. Если любишь «по максимуму» — тогда Pro."
	seedance1LiteText = "Seedance 1 Lite активен.\n\nНапишите описание видео обычным сообщением."
	seedance1ProText  = "Seedance 1 Pro активен.\n\nНапишите описание видео обычным сообщением."

	haiuo02Text         = "🔹 Haiuo 02\n\nHaiuo 02 — картинка суперчёткая, реалистичная, прям как в фильме.\n\nHaiuo 02 Fast — версия «на скорость»: делает видео быстрее, но качество чуть ниже.\n\n☝️ Если нужен «вау-визуал» — бери Haiuo. Если важнее быстро и удобно — Fast."
	haiuo02StandardText = "Haiuo v0.2 Обычный активен.\n\nНапишите описание видео обычным сообщением."
	haiuo02FastText     = "Haiuo v0.2 Fast активен.\n\nНапишите описание видео обычным сообщением."

	studentsText = "🎁Данные нейронные сети помогут вам во время учебы"

	topUpText = "💰 Пополнить баланс\n\nПополнение будет подключено отдельным платежным потоком. Пока для тестирования доступны стартовые кредиты."

	chooseModeText = "Выберите режим в меню выше или нажмите на кнопку показать меню"
)

const (
	photoTextPromptInstruction    = "✅ У вас есть 100 бесплатных попыток в сутки на генерацию с текстом.\n\n▶️ Генерация фото по тексту - это когда вы пишете, что хотите увидеть, а ИИ рисует такую картинку.\n\nНапишите описание обычным сообщением, например: кот в очках на пляже."
	photoTextModeInstruction      = "▶️ Генерация фото по тексту выбрана.\n\nНапишите обычным сообщением, что хотите увидеть.\n\nПример: кот в очках на пляже, кинематографичный свет, высокая детализация"
	photoReferenceModeInstruction = "📸 Генерация фото с референсом пока будет подключена после входящих фото-артефактов.\n\nСейчас доступна генерация фото по тексту."
)

type controlPayload struct {
	Command     string `json:"command"`
	ProductCode string `json:"product_code,omitempty"`
	Action      string `json:"action,omitempty"`
}

func controlPayloadFromPayload(payload string) (controlPayload, bool) {
	if payload == "" {
		return controlPayload{}, false
	}
	var data controlPayload
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return controlPayload{}, false
	}
	t := domain.CommandType(data.Command)
	if !isMenuCommand(t) {
		return controlPayload{}, false
	}
	return data, true
}

func controlTypeFromPayload(payload string) (domain.CommandType, bool) {
	data, ok := controlPayloadFromPayload(payload)
	if !ok {
		return "", false
	}
	t := domain.CommandType(data.Command)
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

type accountView struct {
	Balance               int64
	CompletedGenerations  int
	InvitedCount          int
	RegisteredCount       int
	ActivatedCount        int
	RewardedCount         int
	ReferralLink          string
	ReferrerRewardCredits int64
}

func (h *Handler) sendControlResponse(ctx context.Context, t domain.CommandType, idemKey string, groupID, peerID int64, user *domain.User, allowEdit bool) error {
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

	msgText := screen.text(balance)
	keyboard := screen.keyboard()
	switch t {
	case domain.CommandAccount, domain.CommandBalance:
		view, err := h.accountView(ctx, user.ID, balance, groupID)
		if err != nil {
			return fmt.Errorf("build account view: %w", err)
		}
		msgText = accountDetailsText(view)
		keyboard = accountKeyboard(view)
	case domain.CommandTopUp:
		if pending, ok, err := h.activeTopUpIntent(ctx, user.ID); err != nil {
			return fmt.Errorf("load active top-up intent: %w", err)
		} else if ok {
			msgText = topUpPendingText(pending)
			keyboard = topUpPendingKeyboard(pending.ConfirmationURL)
		} else {
			products, err := h.topUpProducts(ctx)
			if err != nil {
				return fmt.Errorf("load top-up products: %w", err)
			}
			msgText = topUpCatalogText(products)
			keyboard = topUpCatalogKeyboard(products, false)
		}
	}
	markWelcomeSent := user.WelcomeNameSentAt.IsZero() && shouldSendControlResponse(t)
	if t == domain.CommandStart && user.WelcomeNameSentAt.IsZero() {
		if name := h.personalizedWelcomeName(ctx, user); name != "" {
			msgText = welcomeTextWithName(name)
		}
	}

	msg := vkdelivery.Message{
		Text:     msgText,
		Keyboard: keyboard,
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
		if markWelcomeSent {
			user.WelcomeNameSentAt = time.Now()
			if err := h.deps.Users.Update(ctx, user); err != nil {
				return fmt.Errorf("mark welcome sent: %w", err)
			}
		}
		return nil
	}
	return err
}

func (h *Handler) personalizedWelcomeName(ctx context.Context, user *domain.User) string {
	name := strings.TrimSpace(user.VKFirstName)
	if name != "" {
		return name
	}
	if h.deps.Profile == nil {
		return ""
	}

	profile, err := h.deps.Profile.GetUserProfile(ctx, user.VKUserID)
	if err != nil {
		h.logger.Warn("vk user profile lookup failed",
			slog.Int64("vk_user_id", user.VKUserID),
			slog.String("error", err.Error()))
		return ""
	}

	user.VKFirstName = strings.TrimSpace(profile.FirstName)
	user.VKLastName = strings.TrimSpace(profile.LastName)
	user.VKProfileSyncedAt = time.Now()
	if err := h.deps.Users.Update(ctx, user); err != nil {
		h.logger.Warn("vk user profile cache update failed",
			slog.Int64("vk_user_id", user.VKUserID),
			slog.String("error", err.Error()))
	}
	return user.VKFirstName
}

func (h *Handler) accountView(ctx context.Context, userID uuid.UUID, balance, groupID int64) (accountView, error) {
	view := accountView{Balance: balance, ReferrerRewardCredits: h.cfg.ReferralReferrerSignupRewardCredits}
	if h.deps.Jobs != nil {
		count, err := h.deps.Jobs.CountSucceededByUser(ctx, userID)
		if err != nil {
			return view, err
		}
		view.CompletedGenerations = count
	}
	if h.deps.Referrals == nil {
		return view, nil
	}
	code, stats, err := h.deps.Referrals.StatsDetailed(ctx, userID)
	if err != nil {
		return view, err
	}
	view.InvitedCount = stats.Total()
	view.RegisteredCount = stats.RegisteredCount
	view.ActivatedCount = stats.ActivatedCount
	view.RewardedCount = stats.RewardedCount
	view.ReferralLink = buildReferralLink(h.cfg.ReferralLinkBase, groupID, code.Code)
	return view, nil
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
		Text:     chooseModeText,
		Keyboard: menuAccessKeyboard(),
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_unrouted:" + idemKey)
	_, err := h.sendControlMessage(ctx, domain.CommandShowMenu, peerID, randomID, msg)
	return err
}

func (h *Handler) sendTopUpCatalog(ctx context.Context, idemKey string, peerID int64, forceNew, allowEdit bool) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk top-up catalog skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	products, err := h.topUpProducts(ctx)
	if err != nil {
		return fmt.Errorf("load top-up products: %w", err)
	}
	msg := vkdelivery.Message{
		Text:     topUpCatalogText(products),
		Keyboard: topUpCatalogKeyboard(products, forceNew),
	}
	h.applyMenuButtonMode(msg.Keyboard)
	randomID := vkdelivery.DeterministicRandomID(fmt.Sprintf("vk_control_topup_catalog:%s:%t", idemKey, forceNew))
	result, err := h.deliverControlResponse(ctx, domain.CommandTopUp, peerID, randomID, msg, allowEdit)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
	}
	return err
}

func (h *Handler) sendTopUpPaymentLink(ctx context.Context, idemKey string, peerID int64, intent *domain.PaymentIntent) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk top-up payment link skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	link := strings.TrimSpace(intent.ConfirmationURL)
	msg := vkdelivery.Message{
		Text:     fmt.Sprintf("%s СЧЁТ\nПокупка %d генераций\n\nДанная ссылка действительна в течение 10 минут", formatRubAmount(intent.Amount), intent.Credits),
		Keyboard: paymentLinkKeyboard(link),
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_topup_payment:" + idemKey)
	result, err := h.sendControlMessage(ctx, domain.CommandTopUp, peerID, randomID, msg)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
	}
	return err
}

func (h *Handler) sendTopUpNotice(ctx context.Context, idemKey string, peerID int64, text string) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk top-up notice skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	msg := vkdelivery.Message{
		Text:     text,
		Keyboard: backKeyboard(),
	}
	h.applyMenuButtonMode(msg.Keyboard)
	randomID := vkdelivery.DeterministicRandomID(fmt.Sprintf("vk_control_topup_notice:%s:%x", idemKey, hashText(text)))
	_, err := h.sendControlMessage(ctx, domain.CommandTopUp, peerID, randomID, msg)
	return err
}

func (h *Handler) sendGPTPendingMessage(ctx context.Context, idemKey string, peerID int64) int64 {
	if h.deps.Control == nil {
		h.logger.Warn("vk gpt pending message skipped because VK_ACCESS_TOKEN is not configured")
		return 0
	}

	msg := vkdelivery.Message{Text: "НейроХаб думает..."}
	randomID := vkdelivery.DeterministicRandomID("vk_control_gpt_pending:" + idemKey)
	result, err := h.sendControlMessage(ctx, domain.CommandMenuText, peerID, randomID, msg)
	if err != nil {
		h.logger.Warn("vk gpt pending message send failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
		return 0
	}
	return result.MessageID
}

func (h *Handler) sendPhotoPendingMessage(ctx context.Context, idemKey string, peerID int64) int64 {
	if h.deps.Control == nil {
		h.logger.Warn("vk image pending message skipped because VK_ACCESS_TOKEN is not configured")
		return 0
	}

	msg := vkdelivery.Message{Text: "НейроХаб рисует..."}
	randomID := vkdelivery.DeterministicRandomID("vk_control_image_pending:" + idemKey)
	result, err := h.sendControlMessage(ctx, domain.CommandMenuImage, peerID, randomID, msg)
	if err != nil {
		h.logger.Warn("vk image pending message send failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
		return 0
	}
	return result.MessageID
}

func (h *Handler) sendVideoPendingMessage(ctx context.Context, idemKey string, peerID int64) int64 {
	if h.deps.Control == nil {
		h.logger.Warn("vk video pending message skipped because VK_ACCESS_TOKEN is not configured")
		return 0
	}

	msg := vkdelivery.Message{Text: "НейроХаб готовит видео..."}
	randomID := vkdelivery.DeterministicRandomID("vk_control_video_pending:" + idemKey)
	result, err := h.sendControlMessage(ctx, domain.CommandMenuVideo, peerID, randomID, msg)
	if err != nil {
		h.logger.Warn("vk video pending message send failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
		return 0
	}
	return result.MessageID
}

func (h *Handler) editGPTPendingMessage(ctx context.Context, peerID, messageID int64, text string) {
	if h.deps.Control == nil || messageID == 0 {
		return
	}
	if _, err := h.deps.Control.EditMessage(ctx, peerID, messageID, vkdelivery.Message{Text: text}); err != nil {
		h.logger.Warn("vk gpt pending message edit failed",
			slog.Int64("peer_id", peerID),
			slog.Int64("message_id", messageID),
			slog.String("error", err.Error()))
	}
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
	case domain.CommandMenuVideoPrunaAI:
		return h.menuCommandEnabled(domain.CommandMenuVideo)
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
		return false
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

func (h *Handler) hasActiveMenu(peerID int64) bool {
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	_, ok := h.activeMenus[peerID]
	return ok
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
			if keyboard.Buttons[row][col].ActionType == "open_link" {
				continue
			}
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

func welcomeText(_ int64) string {
	return "👋 Добро пожаловать в НейроХаб!\n\n🤖 Здесь вы можете создавать уникальные тексты с помощью нейросети!\n\n📌 Совет: Закрепляй бота, чтобы всегда быть на связи"
}

func welcomeTextWithName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return welcomeText(0)
	}
	return fmt.Sprintf("👋 %s, добро пожаловать в НейроХаб!\n\n🤖 Здесь вы можете создавать уникальные тексты с помощью нейросети!\n\n📌 Совет: Закрепляй бота, чтобы всегда быть на связи", name)
}

func accountText(balance int64) string {
	return fmt.Sprintf("👤 Мой аккаунт\n\nВаш баланс: %d 💎\n\nВыберите действие:", balance)
}

func accountDetailsText(view accountView) string {
	referralLink := view.ReferralLink
	if referralLink == "" {
		referralLink = "ссылка появится после настройки VK_REFERRAL_LINK_BASE"
	}
	return fmt.Sprintf("👤 Мой аккаунт\n\n• безлимитное общение с НейроХаб!\n\n👥 Реферальная программа\n\n• Приглашённых: %d\n• Зарегистрировано: %d\n• Активировано: %d\n• Бонус начислен: %d\n\n• Ссылка: %s\n\nПоддержка: @neirohub_help",
		view.InvitedCount,
		view.RegisteredCount,
		view.ActivatedCount,
		view.RewardedCount,
		referralLink,
	)
}

func (h *Handler) topUpProducts(ctx context.Context) ([]*domain.PaymentProduct, error) {
	if h.deps.Payment == nil {
		return nil, nil
	}
	return h.deps.Payment.ListActiveProducts(ctx)
}

func (h *Handler) activeTopUpIntent(ctx context.Context, userID uuid.UUID) (*domain.PaymentIntent, bool, error) {
	if h.deps.Payment == nil {
		return nil, false, nil
	}
	intent, err := h.deps.Payment.ActiveWaitingIntent(ctx, userID)
	if err == nil {
		return intent, intent != nil, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

func topUpCatalogText(products []*domain.PaymentProduct) string {
	if len(products) == 0 {
		return "💰 Пополнить баланс\n\nТарифы пока недоступны. Попробуйте позже."
	}
	return "Выберите пакет для пополнения баланса:"
}

func topUpPendingText(intent *domain.PaymentIntent) string {
	return fmt.Sprintf("💰 У вас есть незавершенный платеж\n\nПакет: %d кристаллов\nСумма: %s\n\nПосле оплаты баланс обновится автоматически.", intent.Credits, formatRubAmount(intent.Amount))
}

func topUpCatalogKeyboard(products []*domain.PaymentProduct, forceNew bool) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, len(products)+1)
	for _, product := range products {
		if product == nil {
			continue
		}
		rows = append(rows, []vkdelivery.KeyboardButton{
			productButton(topUpProductLabel(product), product.Code, forceNew),
		})
	}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
	}
}

func topUpPendingKeyboard(link string) *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				openLinkButton("Продолжить оплату", link),
			},
			{
				buttonWithAction("Создать новый платеж", domain.CommandTopUp, topUpActionNewPayment, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func topUpProductLabel(product *domain.PaymentProduct) string {
	return fmt.Sprintf("💎 %d кристаллов — %s", product.Credits, formatRubAmount(product.Amount))
}

func formatRubAmount(amount int64) string {
	if amount%100 == 0 {
		return fmt.Sprintf("%d₽", amount/100)
	}
	return fmt.Sprintf("%d.%02d₽", amount/100, amount%100)
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
				button("💬 Спросить у НейроХаб", domain.CommandMenuText, "secondary"),
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
				button("PrunaAI", domain.CommandMenuVideoPrunaAI, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func prunaAIBackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideo)
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

func paymentLinkKeyboard(link string) *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				openLinkButton("Оплатить", link),
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

func emptyAccountKeyboard() *vkdelivery.Keyboard {
	return accountKeyboard(accountView{})
}

func accountKeyboard(view accountView) *vkdelivery.Keyboard {
	rows := [][]vkdelivery.KeyboardButton{}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
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
	payload, _ := json.Marshal(controlPayload{Command: string(command)})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func buttonWithAction(label string, command domain.CommandType, action, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command: string(command),
		Action:  action,
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func productButton(label, productCode string, forceNew bool) vkdelivery.KeyboardButton {
	action := ""
	if forceNew {
		action = topUpActionNewPayment
	}
	payload, _ := json.Marshal(controlPayload{
		Command:     string(domain.CommandTopUp),
		ProductCode: productCode,
		Action:      action,
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   "primary",
	}
}

func openLinkButton(label, link string) vkdelivery.KeyboardButton {
	return vkdelivery.KeyboardButton{
		Label:      label,
		ActionType: "open_link",
		Link:       link,
	}
}

func buildReferralLink(base string, groupID int64, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	base = strings.TrimSpace(base)
	if base == "" {
		if groupID == 0 {
			return ""
		}
		base = fmt.Sprintf("https://vk.com/write-%d", groupID)
	}
	if strings.Contains(base, "{code}") {
		return strings.ReplaceAll(base, "{code}", url.QueryEscape(code))
	}
	return appendURLParam(base, "ref", code)
}

func appendURLParam(raw, key, value string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}
