package accountdelivery

import (
	"context"
	"errors"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/service/accountlink"
	"vk-ai-aggregator/web/platform/branding"
)

func TestPasswordResetLinkTemplateAndDisabledDelivery(t *testing.T) {
	sender, err := NewSender(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(sender.SendPasswordResetLink(context.Background(), "fixture@example.test", "https://platform.example/ru/login/reset#token=synthetic", time.Now()), accountlink.ErrDeliveryUnavailable) {
		t.Fatal("disabled reset sender must fail closed")
	}
	deadline := time.Date(2026, 10, 27, 12, 15, 0, 0, time.UTC)
	link := "https://platform.example/ru/login/reset#token=synthetic"
	message := string(buildPasswordResetMessage("support@example.test", "fixture@example.test", link, deadline))
	parsed, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || !strings.Contains(subject, branding.Name()) {
		t.Fatal("reset subject must identify the current brand")
	}
	if strings.Count(message, link) != 1 || !strings.Contains(message, deadline.Format(time.RFC3339)) || !strings.Contains(message, "проигнорируйте") {
		t.Fatal("reset mail must contain the one-use link, deadline and unsolicited-request guidance")
	}
}
