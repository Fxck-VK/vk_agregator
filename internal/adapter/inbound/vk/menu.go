package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
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
	switch domain.CommandType(data.Command) {
	case domain.CommandStart,
		domain.CommandShowMenu,
		domain.CommandAccount,
		domain.CommandTopUp,
		domain.CommandMenuText,
		domain.CommandMenuImage,
		domain.CommandMenuVideo,
		domain.CommandMenuStudents:
		return domain.CommandType(data.Command), true
	default:
		return "", false
	}
}

func shouldSendControlResponse(t domain.CommandType) bool {
	switch t {
	case domain.CommandStart,
		domain.CommandShowMenu,
		domain.CommandHelp,
		domain.CommandBalance,
		domain.CommandAccount,
		domain.CommandTopUp,
		domain.CommandMenuText,
		domain.CommandMenuImage,
		domain.CommandMenuVideo,
		domain.CommandMenuStudents:
		return true
	default:
		return false
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

	balance := int64(0)
	if t == domain.CommandStart || t == domain.CommandShowMenu || t == domain.CommandHelp || t == domain.CommandBalance || t == domain.CommandAccount {
		acc, err := h.deps.Billing.EnsureAccount(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("ensure billing account: %w", err)
		}
		balance = acc.BalanceCached
	}

	msg := vkdelivery.Message{
		Text:     controlResponseText(t, balance),
		Keyboard: welcomeKeyboard(),
	}
	if t == domain.CommandStart || t == domain.CommandShowMenu || t == domain.CommandHelp {
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

func controlResponseText(t domain.CommandType, balance int64) string {
	switch t {
	case domain.CommandStart, domain.CommandShowMenu, domain.CommandHelp:
		return welcomeText(balance)
	case domain.CommandBalance, domain.CommandAccount:
		return fmt.Sprintf("👤 Мой аккаунт\n\nВаш баланс: %d 💎\n\nВыберите действие:", balance)
	case domain.CommandMenuVideo:
		return "🎬 Создать видео\n\nОтправьте описание после команды /video.\n\nПример:\n/video девушка идет по ночному Токио, cinematic, вертикальный кадр"
	case domain.CommandMenuImage:
		return "🖼️ Создать фото\n\nОтправьте описание после команды /image.\n\nПример:\n/image футуристичный аватар для VK, мягкий свет, детально"
	case domain.CommandMenuText:
		return "💬 Спросить у GPT\n\nНапишите вопрос обычным сообщением.\n\nПример:\nНапиши короткий пост для VK о запуске нового продукта"
	case domain.CommandMenuStudents:
		return "🎁 Студентам и школьникам\n\nМожно просить объяснить тему, составить план, проверить текст или подготовить конспект. Просто напишите задачу обычным сообщением."
	case domain.CommandTopUp:
		return "💰 Пополнить баланс\n\nПополнение будет подключено отдельным платежным потоком. Пока для тестирования доступны стартовые кредиты."
	default:
		return welcomeText(balance)
	}
}

func welcomeText(balance int64) string {
	return fmt.Sprintf("👋 Добро пожаловать в Super GPT!\n\n🤖 Здесь вы можете создавать уникальные тексты, генерировать изображения и видео с помощью нейросетей!\n\nВаш баланс: %d 💎\n🎁 Вам доступна одна бесплатная генерация\n\n📌 Совет: Закрепляй бота и используй промты, которые мы оставили в каждой генеративной модели.", balance)
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
