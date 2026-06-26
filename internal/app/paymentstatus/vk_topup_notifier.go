// Package paymentstatus wires payment lifecycle notifications to user-facing
// app surfaces without putting VK delivery details into paymentservice.
package paymentstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// VKTopUpNotifier edits the VK bot payment-link message after provider-verified
// terminal payment states. It is best-effort: payment and ledger state are
// already committed before this runs.
type VKTopUpNotifier struct {
	Control vkdelivery.ControlClient
	Logger  *slog.Logger
}

// PaymentStatusChanged implements paymentservice.PaymentStatusNotifier.
func (n VKTopUpNotifier) PaymentStatusChanged(ctx context.Context, event paymentservice.PaymentStatusNotification) error {
	if event.Intent == nil || n.Control == nil {
		return nil
	}
	target, ok := vkTopUpStatusTarget(event.To)
	if !ok {
		return nil
	}
	metadata := vkTopUpMetadata(event.Intent.Metadata)
	if metadata.Source != "vk_bot" || metadata.PeerID <= 0 || metadata.MessageID <= 0 {
		return nil
	}
	text := vkTopUpStatusText(event.Intent, target)
	if _, err := n.Control.EditMessage(ctx, metadata.PeerID, metadata.MessageID, vkdelivery.Message{Text: text}); err != nil {
		n.logger().Warn("vk top-up status edit failed",
			slog.String("payment_intent_id", event.Intent.ID.String()),
			slog.String("status", string(event.To)),
			logging.ErrorAttr(err))
		return err
	}
	n.logger().Info("vk top-up status edit completed",
		slog.String("payment_intent_id", event.Intent.ID.String()),
		slog.String("status", string(event.To)))
	return nil
}

func (n VKTopUpNotifier) logger() *slog.Logger {
	if n.Logger != nil {
		return n.Logger
	}
	return slog.Default()
}

type vkTopUpStatus string

const (
	vkTopUpStatusSucceeded vkTopUpStatus = "succeeded"
	vkTopUpStatusDeclined  vkTopUpStatus = "declined"
)

func vkTopUpStatusTarget(status domain.PaymentIntentStatus) (vkTopUpStatus, bool) {
	switch status {
	case domain.PaymentIntentSucceeded:
		return vkTopUpStatusSucceeded, true
	case domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
		return vkTopUpStatusDeclined, true
	default:
		return "", false
	}
}

type vkTopUpLocalMetadata struct {
	Source    string
	PeerID    int64
	MessageID int64
}

func vkTopUpMetadata(raw json.RawMessage) vkTopUpLocalMetadata {
	var metadata struct {
		Source               string `json:"source"`
		VKPeerID             int64  `json:"vk_peer_id"`
		VKPaymentMessageID   int64  `json:"vk_payment_message_id"`
		LegacyMessageID      int64  `json:"vk_message_id"`
		LegacyTopUpMessageID int64  `json:"vk_topup_message_id"`
	}
	_ = json.Unmarshal(raw, &metadata)
	messageID := metadata.VKPaymentMessageID
	if messageID == 0 {
		messageID = metadata.LegacyTopUpMessageID
	}
	if messageID == 0 {
		messageID = metadata.LegacyMessageID
	}
	return vkTopUpLocalMetadata{
		Source:    strings.TrimSpace(metadata.Source),
		PeerID:    metadata.VKPeerID,
		MessageID: messageID,
	}
}

func vkTopUpStatusText(intent *domain.PaymentIntent, status vkTopUpStatus) string {
	switch status {
	case vkTopUpStatusSucceeded:
		return fmt.Sprintf("✅ Пополнение успешно\n\nБаланс пополнен на %d ⭐️.\nСумма: %s.", intent.Credits, formatRubAmount(intent.Amount))
	default:
		return fmt.Sprintf("❌ Платеж отклонен\n\nПокупка %d ⭐️ на сумму %s не завершена.", intent.Credits, formatRubAmount(intent.Amount))
	}
}

func formatRubAmount(amount int64) string {
	if amount%100 == 0 {
		return fmt.Sprintf("%d₽", amount/100)
	}
	return fmt.Sprintf("%d.%02d₽", amount/100, amount%100)
}
