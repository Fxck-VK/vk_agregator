package paymentservice_test

import (
	"context"
	"github.com/google/uuid"
	"net/url"
	"strings"
	"testing"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

type returnRecordingProvider struct {
	*paymentmock.Provider
	returnURL string
}

func (p *returnRecordingProvider) CreatePayment(ctx context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	p.returnURL = in.ReturnURL
	return p.Provider.CreatePayment(ctx, in)
}

func TestAccountPaymentHasIndependentReturnURL(t *testing.T) {
	repo := memory.NewPaymentRepo()
	repo.PutProduct(&domain.PaymentProduct{Code: "stars", Amount: 40000, Currency: domain.CurrencyRUB, Credits: 800, CreditDenominationVersion: 2, IsActive: true})
	provider := &returnRecordingProvider{Provider: paymentmock.New()}
	svc := paymentservice.New(repo, provider, paymentservice.Config{ReturnURL: "https://vk.example.test", AccountReturnURL: "https://web.example.test/app/payment-return"})
	in := paymentservice.CreateAccountIntentInput{AccountID: uuid.New(), ProductCode: "stars", ReceiptEmail: "buyer@example.test", IdempotencyKey: "web-independent-return"}
	first, err := svc.CreateAccountIntent(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(provider.returnURL)
	if u.Query().Get("payment_id") != first.Intent.ID.String() {
		t.Fatal("return page cannot recover its payment without browser storage")
	}
	if !strings.Contains(string(first.Intent.Metadata), `"return_url":"https://web.example.test/app/payment-return"`) {
		t.Fatal("web return was not frozen")
	}
	second, err := svc.CreateAccountIntent(context.Background(), in)
	if err != nil || second.Intent.ID != first.Intent.ID {
		t.Fatal("retry created a different payment")
	}
}
