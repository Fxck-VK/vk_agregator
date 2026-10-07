package accountdelivery

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"time"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountlink"
)

// SendAccountSecurityNotice never accepts arbitrary subject/body content.
func (s *Sender) SendAccountSecurityNotice(ctx context.Context, notice domain.AccountSecurityNotice) error {
	if s == nil {
		return accountlink.ErrDeliveryUnavailable
	}
	smtp, ok := s.email.(*smtpSender)
	if !ok || smtp == nil {
		return accountlink.ErrDeliveryUnavailable
	}
	message, err := buildSecurityNoticeMessage(smtp.cfg.From, notice.Recipient, notice.Kind, notice.EventAt)
	if err != nil {
		return err
	}
	return smtp.sendMessage(ctx, notice.Recipient, message)
}

func buildSecurityNoticeMessage(from, to string, kind domain.AccountSecurityNoticeKind, at time.Time) ([]byte, error) {
	var event string
	switch kind {
	case domain.AccountSecurityNoticePasswordChanged:
		event = "Пароль аккаунта изменён."
	case domain.AccountSecurityNoticePasswordReset:
		event = "Пароль аккаунта сброшен и установлен заново."
	case domain.AccountSecurityNoticeBackupEmailAdded:
		event = "К аккаунту добавлена резервная электронная почта."
	case domain.AccountSecurityNoticeBackupEmailReplaced:
		event = "Резервная электронная почта аккаунта заменена."
	case domain.AccountSecurityNoticeEmailRemoved:
		event = "Электронная почта отвязана от аккаунта."
	default:
		return nil, domain.ErrInvalidIdentity
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n", sanitizeHeader(from), sanitizeHeader(to), mime.QEncoding.Encode("utf-8", "Безопасность аккаунта НейроХаб"))
	fmt.Fprintf(&buf, "НейроХаб: уведомление о безопасности аккаунта.\r\n%s\r\nВремя события (UTC): %s.\r\nЕсли это были не вы, откройте НейроХаб самостоятельно, восстановите доступ и проверьте настройки безопасности.\r\n", event, at.UTC().Format(time.RFC3339))
	return buf.Bytes(), nil
}
