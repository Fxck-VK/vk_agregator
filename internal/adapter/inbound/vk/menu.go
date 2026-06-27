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
	"vk-ai-aggregator/internal/platform/logging"
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
		text:         fixedText(topUpText),
		keyboard:     backKeyboard,
		needsBalance: true,
	},
	domain.CommandMenuText: {
		text:     fixedText(gptActiveText),
		keyboard: backKeyboard,
	},
	domain.CommandMenuImage: {
		text:     fixedText(photoTextPromptInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageSelect: {
		text:     fixedText(photoTextPromptInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageText: {
		text:     fixedText(photoTextModeInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageNanoBanana2: {
		text:     fixedText(photoNanoBanana2Instruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageDeepInfraSeedream: {
		text:     fixedText(photoDeepInfraSeedreamInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageDeepInfraSDXL: {
		text:     fixedText(photoDeepInfraSDXLInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageGPTImage2: {
		text:     fixedText(photoGPTImage2Instruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageQuality1K: {
		text:     fixedText(photoQualityFallbackText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageQuality2K: {
		text:     fixedText(photoQualityFallbackText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageQuality4K: {
		text:     fixedText(photoQualityFallbackText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageQualitySelect: {
		text:     fixedText(photoQualityFallbackText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageBackToQuality: {
		text:     fixedText(photoQualityFallbackText),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuImageReference: {
		text:     fixedText(photoReferenceModeInstruction),
		keyboard: photoModeKeyboard,
	},
	domain.CommandMenuVideo: {
		text:     fixedText("Выбери режим видео:"),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoRouteSelect: {
		text:     fixedText("Выбери режим видео:"),
		keyboard: videoModelKeyboard,
	},
	domain.CommandMenuVideoDurationSelect: {
		text:     fixedText("Выберите длительность видео"),
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
		keyboard: sora2DurationKeyboard,
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
		keyboard: kling21DurationKeyboard,
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
		keyboard: seedance1DurationKeyboard,
	},
	domain.CommandMenuVideoSeedance1Pro: {
		text:     fixedText(seedance1ProText),
		keyboard: seedance1BackKeyboard,
	},
	domain.CommandMenuVideoHailuo02: {
		text:     fixedText(hailuo02Text),
		keyboard: hailuo02Keyboard,
	},
	domain.CommandMenuVideoHailuo02Standard: {
		text:     fixedText(hailuo02StandardText),
		keyboard: hailuo02StandardDurationKeyboard,
	},
	domain.CommandMenuVideoHailuo02Fast: {
		text:     fixedText(hailuo02FastText),
		keyboard: hailuo02FastDurationKeyboard,
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

	prunaAIText = "Видео-режим отключен.\n\nВыберите другой режим видео."

	sora2Text         = "Runway Gen-4 Turbo\n\nРежим для выразительных роликов из стартового фото."
	sora2StartText    = "Runway Gen-4 Turbo активен.\n\nПрикрепите стартовое фото и напишите описание видео одним сообщением.\n\nПример: cinematic camera movement, rain reflections, realistic motion."
	sora2ExamplesText = "ℹ️ Примеры Runway Gen-4 Turbo\n\n1. A cinematic drone shot over a neon city at night, rain reflections on the street, realistic camera movement.\n\n2. A close-up handheld video of a chef cutting fruit in a bright kitchen, natural motion, realistic details."

	kling21Text         = "Kling O3 Standard\n\nУниверсальный режим для видео по тексту или фото."
	kling21StartText    = "Kling O3 Standard активен.\n\nНапишите описание видео обычным сообщением.\n\nПример: warm cinematic scene, friends walking under streetlights, realistic motion."
	kling21ExamplesText = "ℹ️ Примеры Kling O3 Standard\n\n1. A warm cinematic scene of friends walking under streetlights, soft party lights, realistic skin and motion.\n\n2. A product video of a glass bottle rotating on a table, studio lighting, smooth camera movement."

	seedance1Text     = "Seedance 2.0 Fast\n\nРежим для генерации с референсами."
	seedance1LiteText = "Seedance 2.0 Fast активен.\n\nНапишите описание видео обычным сообщением."
	seedance1ProText  = "Seedance 2.0 Pro пока скрыт.\n\nВыберите другой режим видео."

	hailuo02Text         = "Hailuo 2.3 Standard / Hailuo 2.3 Fast\n\nВыберите качество или быстрый режим движения из фото."
	hailuo02StandardText = "Hailuo 2.3 Standard активен.\n\nНапишите описание видео обычным сообщением."
	hailuo02FastText     = "Hailuo 2.3 Fast активен.\n\nПрикрепите стартовое фото и напишите описание видео обычным сообщением."

	studentsText = "🎁Данные нейронные сети помогут вам во время учебы"

	topUpText = "💰 Пополнить баланс\n\nВыберите пакет для пополнения баланса."

	chooseModeText = "Выберите режим в меню выше или нажмите на кнопку показать меню"
)

const (
	photoTextPromptInstruction    = "▶️ Генерация фото по тексту - это когда вы пишете, что хотите увидеть, а ИИ рисует такую картинку\n\nНапишите описание обычным сообщением, например: кот в очках на пляже"
	photoTextModeInstruction      = "▶️ Генерация фото по тексту выбрана.\n\nНапишите обычным сообщением, что хотите увидеть.\n\nПример: кот в очках на пляже, кинематографичный свет, высокая детализация"
	photoReferenceModeInstruction = "📸 Генерация фото с референсом пока будет подключена после входящих фото-артефактов.\n\nСейчас доступна генерация фото по тексту."
)

const photoNanoBanana2Instruction = "Nano Banana 2 активен.\n\nНапишите описание изображения обычным сообщением.\n\nВ боте сейчас включен текст-в-фото; референс-фото подключим отдельным шагом."
const photoDeepInfraSeedreamInstruction = "ByteDance Seedream 4.5 отключен.\n\nВыберите другую модель фото."
const photoDeepInfraSDXLInstruction = "Stability AI SDXL Turbo отключен.\n\nВыберите другую модель фото."
const photoGPTImage2Instruction = "GPT Image 2 активен.\n\nНапишите описание изображения обычным сообщением."
const photoQualityFallbackText = "Выберите модель фото, затем качество генерации."

type controlPayload struct {
	Command         string `json:"command"`
	ProductCode     string `json:"product_code,omitempty"`
	Action          string `json:"action,omitempty"`
	DurationSec     int    `json:"duration_sec,omitempty"`
	ModelID         string `json:"model_id,omitempty"`
	ImageQuality    string `json:"image_quality,omitempty"`
	VideoRouteAlias string `json:"video_route_alias,omitempty"`
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

func usesPhotoModeKeyboard(t domain.CommandType) bool {
	switch t {
	case domain.CommandMenuImage,
		domain.CommandMenuImageSelect,
		domain.CommandMenuImageText,
		domain.CommandMenuImageNanoBanana2,
		domain.CommandMenuImageDeepInfraSeedream,
		domain.CommandMenuImageDeepInfraSDXL,
		domain.CommandMenuImageGPTImage2,
		domain.CommandMenuImageQuality1K,
		domain.CommandMenuImageQuality2K,
		domain.CommandMenuImageQuality4K,
		domain.CommandMenuImageQualitySelect,
		domain.CommandMenuImageBackToQuality,
		domain.CommandMenuImageReference:
		return true
	default:
		return false
	}
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
	if usesPhotoModeKeyboard(t) {
		keyboard = h.photoModeKeyboard()
	}
	if t == domain.CommandMenuVideo {
		keyboard = h.videoModelKeyboard()
	}
	switch t {
	case domain.CommandAccount, domain.CommandBalance:
		view, err := h.accountView(ctx, user.ID, balance, groupID)
		if err != nil {
			return fmt.Errorf("build account view: %w", err)
		}
		msgText = accountDetailsText(view)
		msgText = insertBalanceLine(msgText, view.Balance)
		keyboard = accountKeyboard(view)
	case domain.CommandTopUp:
		returnURL := h.topUpReturnURL(groupID)
		if pending, ok, err := h.activeTopUpIntent(ctx, user.ID, returnURL); err != nil {
			return fmt.Errorf("load active top-up intent: %w", err)
		} else if ok {
			link, ok := h.topUpPaymentLink(pending)
			if !ok {
				msgText = topUpPaymentUnavailableText(balance)
				keyboard = backKeyboard()
			} else {
				msgText = topUpPendingText(balance, pending)
				keyboard = topUpPendingKeyboard(link)
			}
		} else {
			products, err := h.topUpProducts(ctx)
			if err != nil {
				return fmt.Errorf("load top-up products: %w", err)
			}
			msgText = topUpCatalogText(balance, products)
			keyboard = topUpCatalogKeyboard(products, false)
		}
	}
	hasPersistedUser := user != nil && user.ID != uuid.Nil
	markWelcomeSent := hasPersistedUser && user.WelcomeNameSentAt.IsZero() && shouldSendControlResponse(t)
	if t == domain.CommandStart && hasPersistedUser && user.WelcomeNameSentAt.IsZero() {
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
			logging.ErrorAttr(err))
		return ""
	}

	user.VKFirstName = strings.TrimSpace(profile.FirstName)
	user.VKLastName = strings.TrimSpace(profile.LastName)
	user.VKProfileSyncedAt = time.Now()
	if err := h.deps.Users.Update(ctx, user); err != nil {
		h.logger.Warn("vk user profile cache update failed",
			slog.Int64("vk_user_id", user.VKUserID),
			logging.ErrorAttr(err))
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
				logging.ErrorAttr(err))
		}
	}
	return h.sendControlMessage(ctx, t, peerID, randomID, msg)
}

func (h *Handler) editControlMessage(ctx context.Context, t domain.CommandType, peerID, messageID int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	result, err := h.deps.Control.EditMessage(ctx, peerID, messageID, msg)
	if err == nil {
		return result, nil
	}
	if msg.Keyboard != nil && vkdelivery.IsAPIErrorCode(err, 911, 912) {
		h.logger.Warn("vk keyboard edit failed; sending a new control response instead",
			slog.String("command_type", string(t)),
			logging.ErrorAttr(err))
	}
	return vkdelivery.SendResult{}, err
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
		logging.ErrorAttr(err))
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

func (h *Handler) sendTopUpCatalog(ctx context.Context, idemKey string, peerID int64, user *domain.User, forceNew, allowEdit bool) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk top-up catalog skipped because VK_ACCESS_TOKEN is not configured")
		return nil
	}
	balance, err := h.currentBalance(ctx, user)
	if err != nil {
		return err
	}
	products, err := h.topUpProducts(ctx)
	if err != nil {
		return fmt.Errorf("load top-up products: %w", err)
	}
	msg := vkdelivery.Message{
		Text:     topUpCatalogText(balance, products),
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

func (h *Handler) sendTopUpPaymentLink(ctx context.Context, idemKey string, peerID int64, balance int64, intent *domain.PaymentIntent) (int64, error) {
	if h.deps.Control == nil {
		h.logger.Warn("vk top-up payment link skipped because VK_ACCESS_TOKEN is not configured")
		return 0, nil
	}
	link, ok := h.topUpPaymentLink(intent)
	if !ok {
		return 0, h.sendTopUpNotice(ctx, idemKey, peerID, topUpPaymentUnavailableText(balance))
	}
	msg := vkdelivery.Message{
		Text: fmt.Sprintf("%s СЧЕТ\n\nПокупка: %d ⭐️\nБаланс сейчас: %d ⭐️\nПосле оплаты: %d ⭐️\n\nОткройте оплату кнопкой ниже\nСсылка на оплату действует ограниченное время",
			formatRubAmount(intent.Amount),
			intent.Credits,
			balance,
			balance+intent.Credits,
		),
		Keyboard: paymentLinkKeyboard(link),
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_topup_payment:" + idemKey)
	result, err := h.sendControlMessage(ctx, domain.CommandTopUp, peerID, randomID, msg)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
	}
	return result.MessageID, err
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
			logging.ErrorAttr(err))
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
			logging.ErrorAttr(err))
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
			logging.ErrorAttr(err))
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
			logging.ErrorAttr(err))
	}
}

func (h *Handler) showInsufficientBalanceMessage(ctx context.Context, idemKey string, peerID, messageID int64) {
	if h.deps.Control == nil {
		return
	}
	msg := vkdelivery.Message{
		Text:     "Недостаточный баланс",
		Keyboard: insufficientBalanceKeyboard(),
	}
	h.applyMenuButtonMode(msg.Keyboard)
	if messageID != 0 {
		if _, err := h.deps.Control.EditMessage(ctx, peerID, messageID, msg); err == nil {
			return
		} else {
			h.logger.Warn("vk insufficient balance message edit failed",
				slog.Int64("peer_id", peerID),
				slog.Int64("message_id", messageID),
				logging.ErrorAttr(err))
		}
	}
	randomID := vkdelivery.DeterministicRandomID("vk_control_insufficient_balance:" + idemKey)
	if _, err := h.sendControlMessage(ctx, domain.CommandTopUp, peerID, randomID, msg); err != nil {
		h.logger.Warn("vk insufficient balance message send failed",
			slog.Int64("peer_id", peerID),
			logging.ErrorAttr(err))
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
			control, ok := controlPayloadFromPayload(button.Payload)
			if ok && !h.controlPayloadEnabled(control) {
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
	case domain.CommandMenuVideoRouteSelect,
		domain.CommandMenuVideoDurationSelect:
		return h.menuCommandEnabled(domain.CommandMenuVideo)
	case domain.CommandMenuVideoPrunaAI:
		return false
	case domain.CommandMenuVideoSora2,
		domain.CommandMenuVideoKling21,
		domain.CommandMenuVideoSeedance1,
		domain.CommandMenuVideoHailuo02:
		return h.videoRouteCommandEnabled(command) && h.menuCommandEnabled(domain.CommandMenuVideo)
	case domain.CommandMenuVideoSora2Start,
		domain.CommandMenuVideoSora2Examples:
		return h.videoRouteCommandEnabled(command) && h.menuCommandEnabled(domain.CommandMenuVideoSora2)
	case domain.CommandMenuVideoKling21Start,
		domain.CommandMenuVideoKling21Examples:
		return h.videoRouteCommandEnabled(command) && h.menuCommandEnabled(domain.CommandMenuVideoKling21)
	case domain.CommandMenuVideoSeedance1Lite,
		domain.CommandMenuVideoSeedance1Pro:
		return h.videoRouteCommandEnabled(command) && h.menuCommandEnabled(domain.CommandMenuVideoSeedance1)
	case domain.CommandMenuVideoHailuo02Standard,
		domain.CommandMenuVideoHailuo02Fast:
		return h.videoRouteCommandEnabled(command) && h.menuCommandEnabled(domain.CommandMenuVideoHailuo02)
	case domain.CommandMenuImageSelect,
		domain.CommandMenuImageText,
		domain.CommandMenuImageNanoBanana2,
		domain.CommandMenuImageDeepInfraSeedream,
		domain.CommandMenuImageDeepInfraSDXL,
		domain.CommandMenuImageGPTImage2,
		domain.CommandMenuImageQuality1K,
		domain.CommandMenuImageQuality2K,
		domain.CommandMenuImageQuality4K,
		domain.CommandMenuImageQualitySelect,
		domain.CommandMenuImageBackToQuality,
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

func (h *Handler) controlPayloadEnabled(control controlPayload) bool {
	command := domain.CommandType(control.Command)
	if !h.menuCommandEnabled(command) {
		return false
	}
	switch command {
	case domain.CommandMenuImageSelect:
		return h.publicImageModelEnabled(control.ModelID)
	case domain.CommandMenuImageQualitySelect:
		modelID := strings.TrimSpace(control.ModelID)
		if modelID == "" {
			return true
		}
		return h.publicImageQualityAllowed(modelID, control.ImageQuality)
	case domain.CommandMenuVideoRouteSelect:
		return h.publicVideoRouteEnabled(control.VideoRouteAlias)
	case domain.CommandMenuVideoDurationSelect:
		return h.publicVideoRouteDurationAllowed(control.VideoRouteAlias, control.DurationSec)
	default:
		return true
	}
}

func (h *Handler) videoRouteCommandEnabled(command domain.CommandType) bool {
	return h.cfg.MenuFeatures.EnabledCommands[command]
}

func (h *Handler) getActiveMenu(peerID int64) (activeMenuMessage, bool) {
	now := time.Now()
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	h.pruneActiveMenusLocked(now, peerID)
	msg, ok := h.activeMenus[peerID]
	return msg, ok
}

func (h *Handler) hasActiveMenu(peerID int64) bool {
	now := time.Now()
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	h.pruneActiveMenusLocked(now, peerID)
	_, ok := h.activeMenus[peerID]
	return ok
}

func (h *Handler) setActiveMenu(peerID, messageID int64) {
	if messageID == 0 {
		return
	}
	now := time.Now()
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	h.activeMenus[peerID] = activeMenuMessage{
		MessageID: messageID,
		ExpiresAt: now.Add(h.cfg.LocalUIStateTTL),
	}
	h.pruneActiveMenusLocked(now, peerID)
}

func (h *Handler) clearActiveMenu(peerID int64) {
	h.menuMu.Lock()
	defer h.menuMu.Unlock()
	delete(h.activeMenus, peerID)
}

func (h *Handler) pruneActiveMenusLocked(now time.Time, keepPeerID int64) {
	for peerID, msg := range h.activeMenus {
		if msg.MessageID == 0 || localUIStateExpired(msg.ExpiresAt, now) {
			delete(h.activeMenus, peerID)
		}
	}
	for len(h.activeMenus) > h.cfg.LocalUIStateMaxEntries {
		removed := false
		for peerID := range h.activeMenus {
			if peerID == keepPeerID && len(h.activeMenus) > 1 {
				continue
			}
			delete(h.activeMenus, peerID)
			removed = true
			break
		}
		if !removed {
			return
		}
	}
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
			logging.ErrorAttr(err))
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
	return fmt.Sprintf("👤 Мой аккаунт\n\nВаш баланс: %d ⭐️\n\nВыберите действие:", balance)
}

func accountDetailsText(view accountView) string {
	referralLink := view.ReferralLink
	if referralLink == "" {
		referralLink = "ссылка появится после настройки VK_REFERRAL_LINK_BASE"
	}
	return fmt.Sprintf("👤 Мой аккаунт\n\n• общение с НейроХаб\n\n👥 Реферальная программа\n\n• Приглашённых: %d\n• Зарегистрировано: %d\n• Активировано: %d\n• Бонус начислен: %d\n\n• Ссылка: %s\n\nПоддержка: @neirohub_help",
		view.InvitedCount,
		view.RegisteredCount,
		view.ActivatedCount,
		view.RewardedCount,
		referralLink,
	)
}

func insertBalanceLine(text string, balance int64) string {
	line := fmt.Sprintf("Баланс: %d ⭐️", balance)
	if strings.Contains(text, line) {
		return text
	}
	return strings.Replace(text, "\n\n", "\n\n"+line+"\n\n", 1)
}

func (h *Handler) topUpProducts(ctx context.Context) ([]*domain.PaymentProduct, error) {
	if h.deps.Payment == nil {
		return nil, nil
	}
	return h.deps.Payment.ListActiveProducts(ctx)
}

func (h *Handler) activeTopUpIntent(ctx context.Context, userID uuid.UUID, returnURL string) (*domain.PaymentIntent, bool, error) {
	if h.deps.Payment == nil {
		return nil, false, nil
	}
	intent, err := h.deps.Payment.ActiveWaitingIntentForSource(ctx, userID, "vk_bot")
	if err == nil {
		if !paymentIntentReturnURLMatches(intent, returnURL) {
			return nil, false, nil
		}
		return intent, intent != nil, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

func topUpCatalogText(balance int64, products []*domain.PaymentProduct) string {
	if len(products) == 0 {
		return fmt.Sprintf("💰 Пополнить баланс\n\nБаланс сейчас: %d ⭐️\n\nТарифы пока недоступны. Попробуйте позже.", balance)
	}
	return fmt.Sprintf("💰 Пополнить баланс\n\nБаланс сейчас: %d ⭐️\n\nВыберите пакет для пополнения баланса:", balance)
}

func topUpPendingText(balance int64, intent *domain.PaymentIntent) string {
	return fmt.Sprintf("💰 У вас есть незавершенный платеж\n\nБаланс сейчас: %d ⭐️\nПакет: %d ⭐️\nСумма: %s\nПосле оплаты: %d ⭐️\n\nПродолжите оплату кнопкой ниже\nПосле оплаты баланс обновится автоматически", balance, intent.Credits, formatRubAmount(intent.Amount), balance+intent.Credits)
}

func topUpPaymentUnavailableText(balance int64) string {
	return fmt.Sprintf("💰 Пополнить баланс\n\nБаланс сейчас: %d ⭐️\n\nПлатежи временно недоступны. Попробуйте позже.", balance)
}

func topUpCatalogKeyboard(products []*domain.PaymentProduct, forceNew bool) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, len(products)+1)
	compact := len(products) > 5
	var row []vkdelivery.KeyboardButton
	for _, product := range products {
		if product == nil {
			continue
		}
		row = append(row, productButton(topUpProductLabel(product), product.Code, forceNew))
		if !compact || len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
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

func (h *Handler) topUpPaymentLink(intent *domain.PaymentIntent) (string, bool) {
	if intent == nil || intent.ID == uuid.Nil {
		return "", false
	}
	u, ok := h.topUpPaymentRedirectBase()
	if !ok {
		return "", false
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/payments/vk/" + intent.ID.String()
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), true
}

func (h *Handler) topUpPaymentRedirectConfigured() bool {
	_, ok := h.topUpPaymentRedirectBase()
	return ok
}

func (h *Handler) topUpPaymentRedirectBase() (*url.URL, bool) {
	base := strings.TrimSpace(h.cfg.TopUpPaymentRedirectBaseURL)
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, false
	}
	return u, true
}

func topUpProductLabel(product *domain.PaymentProduct) string {
	return fmt.Sprintf("%d ⭐️ — %s", product.Credits, formatRubAmount(product.Amount))
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

// videoModelKeyboard is a legacy fallback for stale command screens. The
// primary VK video menu is Handler.videoModelKeyboard and uses catalog payloads.
func videoModelKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Hailuo 2.3 Fast", domain.CommandMenuVideoHailuo02Fast, "secondary"),
			},
			{
				button("Hailuo 2.3 Standard", domain.CommandMenuVideoHailuo02Standard, "secondary"),
			},
			{
				button("Kling O3 Standard", domain.CommandMenuVideoKling21Start, "secondary"),
			},
			{
				button("Seedance 2.0 Fast", domain.CommandMenuVideoSeedance1Lite, "secondary"),
			},
			{
				button("Runway Gen-4 Turbo", domain.CommandMenuVideoSora2Start, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func (h *Handler) videoModelKeyboard() *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, len(h.cfg.VideoRoutes)+1)
	for _, route := range h.cfg.VideoRoutes {
		if !route.Enabled || strings.TrimSpace(route.Alias) == "" || strings.TrimSpace(route.Name) == "" {
			continue
		}
		rows = append(rows, []vkdelivery.KeyboardButton{
			videoRouteButton(route.Name, route.Alias, "primary"),
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

func prunaAIBackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideo)
}

func sora2Keyboard() *vkdelivery.Keyboard {
	return videoDetailKeyboard(domain.CommandMenuVideoSora2Start, domain.CommandMenuVideoSora2Examples)
}

func sora2BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoSora2)
}

func sora2DurationKeyboard() *vkdelivery.Keyboard {
	return videoDurationKeyboard(domain.CommandMenuVideoSora2Start, domain.CommandMenuVideoSora2, 3, 5, 10)
}

func kling21Keyboard() *vkdelivery.Keyboard {
	return videoDetailKeyboard(domain.CommandMenuVideoKling21Start, domain.CommandMenuVideoKling21Examples)
}

func kling21BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoKling21)
}

func kling21DurationKeyboard() *vkdelivery.Keyboard {
	return videoDurationKeyboard(domain.CommandMenuVideoKling21Start, domain.CommandMenuVideoKling21, 5, 10)
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
				button("Seedance 2.0 Fast", domain.CommandMenuVideoSeedance1Lite, "secondary"),
			},
			{
				button("Seedance 2.0 Pro", domain.CommandMenuVideoSeedance1Pro, "secondary"),
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

func seedance1DurationKeyboard() *vkdelivery.Keyboard {
	return videoDurationKeyboard(domain.CommandMenuVideoSeedance1Lite, domain.CommandMenuVideoSeedance1, 5, 10)
}

func hailuo02Keyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Hailuo 2.3 Standard", domain.CommandMenuVideoHailuo02Standard, "secondary"),
			},
			{
				button("Hailuo 2.3 Fast", domain.CommandMenuVideoHailuo02Fast, "secondary"),
			},
			{
				button("⬅️ Назад", domain.CommandMenuVideo, "secondary"),
			},
		},
	}
}

func hailuo02BackKeyboard() *vkdelivery.Keyboard {
	return backToKeyboard(domain.CommandMenuVideoHailuo02)
}

func hailuo02StandardDurationKeyboard() *vkdelivery.Keyboard {
	return videoDurationKeyboard(domain.CommandMenuVideoHailuo02Standard, domain.CommandMenuVideoHailuo02, 6, 10)
}

func hailuo02FastDurationKeyboard() *vkdelivery.Keyboard {
	return videoDurationKeyboard(domain.CommandMenuVideoHailuo02Fast, domain.CommandMenuVideoHailuo02, 6, 10)
}

func videoDurationKeyboard(startCommand, backCommand domain.CommandType, durations ...int) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, 3)
	durationRow := make([]vkdelivery.KeyboardButton, 0, len(durations))
	for _, duration := range durations {
		durationRow = append(durationRow, durationButton(fmt.Sprintf("%d сек", duration), startCommand, duration, "primary"))
	}
	if len(durationRow) > 0 {
		rows = append(rows, durationRow)
	}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад", backCommand, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
	}
}

func (h *Handler) videoRouteDurationKeyboard(spec videoModeSpec) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, 4)
	durationRow := make([]vkdelivery.KeyboardButton, 0, 3)
	for _, duration := range spec.AllowedDurationsSec {
		if duration <= 0 {
			continue
		}
		pricedSpec := spec
		pricedSpec.DurationSec = duration
		price, ok := h.videoDisplayEstimateCredits(pricedSpec)
		if !ok {
			continue
		}
		durationRow = append(durationRow, videoDurationButton(fmt.Sprintf("%d сек · %d ⭐️", duration, price), string(spec.VideoRouteAlias), duration, "primary"))
		if len(durationRow) == 3 {
			rows = append(rows, durationRow)
			durationRow = make([]vkdelivery.KeyboardButton, 0, 3)
		}
	}
	if len(durationRow) > 0 {
		rows = append(rows, durationRow)
	}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад к видео", domain.CommandMenuVideo, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
	}
}

// photoModeKeyboard is a legacy fallback for stale command screens. The primary
// VK image menu is Handler.photoModeKeyboard and uses catalog payloads.
func photoModeKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Nano Banana 2", domain.CommandMenuImageNanoBanana2, "primary"),
			},
			{
				button("Nano Banana Pro", domain.CommandMenuImageText, "primary"),
			},
			{
				button("GPT Image 2", domain.CommandMenuImageGPTImage2, "primary"),
			},
			{
				button("⬅️ Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func (h *Handler) photoModeKeyboard() *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, len(h.cfg.ImageModels)+1)
	for _, model := range h.cfg.ImageModels {
		if !model.Enabled || strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Name) == "" {
			continue
		}
		rows = append(rows, []vkdelivery.KeyboardButton{
			photoModelButton(model.Name, model.ID, "primary"),
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

type photoQualityOption struct {
	Label   string
	Price   int64
	Command domain.CommandType
	ModelID string
	Quality string
}

func photoQualityKeyboard(options []photoQualityOption) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, len(options)+1)
	for _, option := range options {
		label := fmt.Sprintf("%s · %d ⭐️", option.Label, option.Price)
		rows = append(rows, []vkdelivery.KeyboardButton{
			photoQualityButton(label, option.ModelID, option.Quality, "primary"),
		})
	}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад к фото", domain.CommandMenuImage, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
	}
}

func photoPromptKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("⬅️ Назад к качеству", domain.CommandMenuImageBackToQuality, "secondary"),
			},
			{
				button("⬅️ Назад к фото", domain.CommandMenuImage, "secondary"),
			},
		},
	}
}

func photoPromptKeyboardForCatalog(showQualityBack bool) *vkdelivery.Keyboard {
	rows := make([][]vkdelivery.KeyboardButton, 0, 2)
	if showQualityBack {
		rows = append(rows, []vkdelivery.KeyboardButton{
			button("⬅️ Назад к качеству", domain.CommandMenuImageBackToQuality, "secondary"),
		})
	}
	rows = append(rows, []vkdelivery.KeyboardButton{
		button("⬅️ Назад к фото", domain.CommandMenuImage, "secondary"),
	})
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: rows,
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

func insufficientBalanceKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("Пополнить", domain.CommandTopUp, "positive"),
			},
			{
				button("Назад", domain.CommandShowMenu, "secondary"),
			},
		},
	}
}

func videoPromptBackKeyboard() *vkdelivery.Keyboard {
	return &vkdelivery.Keyboard{
		OneTime: false,
		Inline:  true,
		Buttons: [][]vkdelivery.KeyboardButton{
			{
				button("⬅️ Назад к видео", domain.CommandMenuVideo, "secondary"),
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

func photoModelButton(label, modelID, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command: string(domain.CommandMenuImageSelect),
		ModelID: strings.TrimSpace(modelID),
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func photoQualityButton(label, modelID, quality, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command:      string(domain.CommandMenuImageQualitySelect),
		ModelID:      strings.TrimSpace(modelID),
		ImageQuality: strings.TrimSpace(quality),
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func videoRouteButton(label, routeAlias, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command:         string(domain.CommandMenuVideoRouteSelect),
		VideoRouteAlias: strings.TrimSpace(routeAlias),
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func videoDurationButton(label, routeAlias string, durationSec int, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command:         string(domain.CommandMenuVideoDurationSelect),
		VideoRouteAlias: strings.TrimSpace(routeAlias),
		DurationSec:     durationSec,
	})
	return vkdelivery.KeyboardButton{
		Label:   label,
		Payload: string(payload),
		Color:   color,
	}
}

func durationButton(label string, command domain.CommandType, durationSec int, color string) vkdelivery.KeyboardButton {
	payload, _ := json.Marshal(controlPayload{
		Command:     string(command),
		DurationSec: durationSec,
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
