package paymentstatus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

func TestVKTopUpNotifierEditsOnlyTrackedVKBotMessage(t *testing.T) {
	ctx := context.Background()
	control := vkdelivery.NewMockClient()
	sent, err := control.SendMessage(ctx, 777, 1, vkdelivery.Message{
		Text:     "payment link",
		Keyboard: &vkdelivery.Keyboard{Inline: true},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	metadata, _ := json.Marshal(map[string]any{
		"source":                "vk_bot",
		"vk_peer_id":            int64(777),
		"vk_payment_message_id": sent.MessageID,
	})
	intent := &domain.PaymentIntent{
		ID:       uuid.New(),
		Metadata: metadata,
		Status:   domain.PaymentIntentSucceeded,
		Amount:   9900,
		Credits:  99,
	}
	notifier := VKTopUpNotifier{Control: control}
	if err := notifier.PaymentStatusChanged(ctx, paymentservice.PaymentStatusNotification{
		Intent: intent,
		From:   domain.PaymentIntentWaitingForUser,
		To:     domain.PaymentIntentSucceeded,
	}); err != nil {
		t.Fatalf("notify status: %v", err)
	}
	edits := control.Edits()
	if len(edits) != 1 {
		t.Fatalf("edits = %d, want 1", len(edits))
	}
	if !strings.Contains(edits[0].Text, "Пополнение успешно") || edits[0].Keyboard != "" {
		t.Fatalf("unexpected edit: %+v", edits[0])
	}

	miniAppMetadata, _ := json.Marshal(map[string]any{
		"source":                "vk_miniapp",
		"vk_peer_id":            int64(777),
		"vk_payment_message_id": sent.MessageID,
	})
	intent.Metadata = miniAppMetadata
	if err := notifier.PaymentStatusChanged(ctx, paymentservice.PaymentStatusNotification{
		Intent: intent,
		From:   domain.PaymentIntentWaitingForUser,
		To:     domain.PaymentIntentCanceled,
	}); err != nil {
		t.Fatalf("notify miniapp status: %v", err)
	}
	if len(control.Edits()) != 1 {
		t.Fatalf("miniapp payment should not be edited")
	}
}
