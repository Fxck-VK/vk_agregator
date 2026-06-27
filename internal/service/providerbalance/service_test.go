package providerbalance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceHandleBalancesRendersAPIMartValues(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{
		name: "apimart",
		balances: []ProviderBalance{{
			Provider:      "apimart",
			RemainBalance: 88.58,
			RemainCredits: 885.8,
			UsedBalance:   12.34,
			UsedCredits:   123.4,
			CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
		}},
	}
	svc := New([]Checker{checker}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balances"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg,
		"Балансы провайдеров",
		"APIMart",
		"Остаток: 88.58 balance",
		"Остаток credits: 885.8",
		"Использовано: 12.34 balance",
		"Использовано credits: 123.4",
		"Статус: ok",
		"Обновлено: 2026-06-27 18:30 MSK",
	)
}

func TestServiceHandleBalanceAPIMartRendersOnlyAPIMart(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 20, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 99, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance apimart"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg, "APIMart", "Остаток: 20.00 balance")
	if strings.Contains(msg, "PoYo") || poyo.calls != 0 {
		t.Fatalf("expected only APIMart balance, got msg=%q poyo.calls=%d", msg, poyo.calls)
	}
}

func TestServiceHandleBalancePoYoRendersOnlyPoYo(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 20, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 17276, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance poyo"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg, "PoYo", "Остаток credits: 17276.0")
	if strings.Contains(msg, "APIMart") || apimart.calls != 0 {
		t.Fatalf("expected only PoYo balance, got msg=%q apimart.calls=%d", msg, apimart.calls)
	}
}

func TestServiceCachesProviderBalanceWithinTTL(t *testing.T) {
	messenger := &recordingMessenger{}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{
		Provider:      "poyo",
		RemainCredits: 17276,
		CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
	}}}
	svc := New([]Checker{poyo}, messenger, Config{CacheTTL: 5 * time.Minute, Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance poyo"); err != nil {
		t.Fatalf("HandleCommand first returned error: %v", err)
	}
	if err := svc.HandleCommand(context.Background(), "/balance poyo"); err != nil {
		t.Fatalf("HandleCommand second returned error: %v", err)
	}

	if poyo.calls != 1 {
		t.Fatalf("checker calls = %d, want 1", poyo.calls)
	}
	if len(messenger.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messenger.messages))
	}
	assertContainsAll(t, messenger.messages[1], "PoYo", "17276.0")
}

func TestServiceHandleBalanceRunwayRendersOnlyRunway(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 20, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 17276, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	runway := &stubChecker{name: "runway", balances: []ProviderBalance{{Provider: "runway", RemainCredits: 1000, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo, runway}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance runway"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg, "Runway", "Остаток credits: 1000.0")
	if strings.Contains(msg, "APIMart") || strings.Contains(msg, "PoYo") || apimart.calls != 0 || poyo.calls != 0 {
		t.Fatalf("expected only Runway balance, got msg=%q apimart.calls=%d poyo.calls=%d", msg, apimart.calls, poyo.calls)
	}
}

func TestServiceHandleBalanceDeepInfraRendersOnlyDeepInfra(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 20, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 17276, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	runway := &stubChecker{name: "runway", balances: []ProviderBalance{{Provider: "runway", RemainCredits: 1000, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	deepinfra := &stubChecker{name: "deepinfra", balances: []ProviderBalance{{Provider: "deepinfra", RemainBalance: 25.5, UsedBalance: 123, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo, runway, deepinfra}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balance deepinfra"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg, "DeepInfra", "Остаток: 25.50 balance", "Использовано: 123.00 balance")
	if strings.Contains(msg, "APIMart") || strings.Contains(msg, "PoYo") || strings.Contains(msg, "Runway") ||
		apimart.calls != 0 || poyo.calls != 0 || runway.calls != 0 {
		t.Fatalf("expected only DeepInfra balance, got msg=%q apimart.calls=%d poyo.calls=%d runway.calls=%d", msg, apimart.calls, poyo.calls, runway.calls)
	}
}

func TestServiceHandleBalancesRendersAPIMartAndPoYo(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 88.58, RemainCredits: 885.8, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 17276, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balances"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	assertContainsAll(t, messenger.last(t),
		"APIMart",
		"Остаток: 88.58 balance",
		"PoYo",
		"Остаток credits: 17276.0",
	)
}

func TestServiceHandleBalancesRendersAllConfiguredProviders(t *testing.T) {
	messenger := &recordingMessenger{}
	apimart := &stubChecker{name: "apimart", balances: []ProviderBalance{{Provider: "apimart", RemainBalance: 88.58, RemainCredits: 885.8, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	poyo := &stubChecker{name: "poyo", balances: []ProviderBalance{{Provider: "poyo", RemainCredits: 17276, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	runway := &stubChecker{name: "runway", balances: []ProviderBalance{{Provider: "runway", RemainCredits: 1000, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	deepinfra := &stubChecker{name: "deepinfra", balances: []ProviderBalance{{Provider: "deepinfra", RemainBalance: 25.5, UsedBalance: 123, CheckedAt: time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC)}}}
	svc := New([]Checker{apimart, poyo, runway, deepinfra}, messenger, Config{Location: mustMoscow(t)})

	if err := svc.HandleCommand(context.Background(), "/balances"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	assertContainsAll(t, messenger.last(t),
		"APIMart",
		"PoYo",
		"Runway",
		"Остаток credits: 1000.0",
		"DeepInfra",
		"Остаток: 25.50 balance",
	)
}

func TestServiceHelpAndUnknownCommandRenderHelp(t *testing.T) {
	for _, command := range []string{"/help", "/unknown"} {
		t.Run(command, func(t *testing.T) {
			messenger := &recordingMessenger{}
			svc := New(nil, messenger, Config{})

			if err := svc.HandleCommand(context.Background(), command); err != nil {
				t.Fatalf("HandleCommand returned error: %v", err)
			}

			msg := messenger.last(t)
			assertContainsAll(t, msg, "/balances", "/balance apimart", "/balance poyo", "/balance runway", "/balance deepinfra", "/help")
			if strings.Contains(msg, "/usage") {
				t.Fatalf("help must not contain usage command: %s", msg)
			}
		})
	}
}

func TestServiceCheckAndWarnSendsLowBalanceWarning(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{name: "apimart", balances: []ProviderBalance{{
		Provider:      "apimart",
		RemainBalance: 18.5,
		RemainCredits: 500,
		CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
	}}}
	svc := New([]Checker{checker}, messenger, Config{WarnRemainBalance: 20, WarnRemainCredits: 200, Location: mustMoscow(t)})

	if err := svc.CheckAndWarn(context.Background()); err != nil {
		t.Fatalf("CheckAndWarn returned error: %v", err)
	}

	assertContainsAll(t, messenger.last(t),
		"Низкий баланс провайдера",
		"APIMart",
		"Остаток: 18.50 balance",
		"Порог: 20 balance",
	)
}

func TestServiceCheckAndWarnSendsLowCreditsWarning(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{name: "apimart", balances: []ProviderBalance{{
		Provider:      "apimart",
		RemainBalance: 100,
		RemainCredits: 150,
		CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
	}}}
	svc := New([]Checker{checker}, messenger, Config{WarnRemainBalance: 20, WarnRemainCredits: 200, Location: mustMoscow(t)})

	if err := svc.CheckAndWarn(context.Background()); err != nil {
		t.Fatalf("CheckAndWarn returned error: %v", err)
	}

	assertContainsAll(t, messenger.last(t),
		"Низкий баланс провайдера",
		"APIMart",
		"Остаток credits: 150.0",
		"Порог: 200 credits",
	)
}

func TestServiceCheckAndWarnSendsLowDeepInfraBalanceWarning(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{name: "deepinfra", balances: []ProviderBalance{{
		Provider:      "deepinfra",
		RemainBalance: -5,
		UsedBalance:   123,
		CheckedAt:     time.Date(2026, 6, 27, 15, 30, 0, 0, time.UTC),
	}}}
	svc := New([]Checker{checker}, messenger, Config{WarnRemainBalance: 20, Location: mustMoscow(t)})

	if err := svc.CheckAndWarn(context.Background()); err != nil {
		t.Fatalf("CheckAndWarn returned error: %v", err)
	}

	assertContainsAll(t, messenger.last(t),
		"Низкий баланс провайдера",
		"DeepInfra",
		"Остаток: -5.00 balance",
	)
}

func TestServiceCheckAndWarnSuppressesDuplicatesUntilRecovery(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{name: "apimart", balances: []ProviderBalance{
		{Provider: "apimart", RemainBalance: 18.5, RemainCredits: 500},
		{Provider: "apimart", RemainBalance: 18.0, RemainCredits: 500},
		{Provider: "apimart", RemainBalance: 25.0, RemainCredits: 500},
		{Provider: "apimart", RemainBalance: 19.0, RemainCredits: 500},
	}}
	svc := New([]Checker{checker}, messenger, Config{WarnRemainBalance: 20, WarnRemainCredits: 200})

	for i := 0; i < 4; i++ {
		if err := svc.CheckAndWarn(context.Background()); err != nil {
			t.Fatalf("CheckAndWarn %d returned error: %v", i, err)
		}
	}
	if len(messenger.messages) != 2 {
		t.Fatalf("warnings sent = %d, want 2: %#v", len(messenger.messages), messenger.messages)
	}
}

func TestServiceProviderErrorRendersSanitizedError(t *testing.T) {
	messenger := &recordingMessenger{}
	checker := &stubChecker{
		name: "apimart",
		errs: []error{errors.New("APIMart balance API returned 401 with Authorization: Bearer secret-token at https://private.example/path")},
	}
	svc := New([]Checker{checker}, messenger, Config{})

	if err := svc.HandleCommand(context.Background(), "/balance apimart"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg,
		"Не удалось получить баланс APIMart",
		"Статус: provider_unavailable",
		"Ошибка:",
	)
	for _, forbidden := range []string{"secret-token", "https://private.example/path", "Authorization"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("message leaked %q: %s", forbidden, msg)
		}
	}
}

func TestServicePoYoProviderErrorRendersSanitizedError(t *testing.T) {
	messenger := &recordingMessenger{}
	testEmail := "owner" + "@" + "example.test"
	checker := &stubChecker{
		name: "poyo",
		errs: []error{errors.New("PoYo balance API returned 401 with Authorization: Bearer secret-token for " + testEmail + " at https://private.poyo.example/path")},
	}
	svc := New([]Checker{checker}, messenger, Config{})

	if err := svc.HandleCommand(context.Background(), "/balance poyo"); err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	msg := messenger.last(t)
	assertContainsAll(t, msg,
		"Не удалось получить баланс PoYo",
		"Статус: provider_unavailable",
		"Ошибка:",
	)
	for _, forbidden := range []string{"secret-token", testEmail, "https://private.poyo.example/path", "Authorization"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("message leaked %q: %s", forbidden, msg)
		}
	}
}

type stubChecker struct {
	name     string
	balances []ProviderBalance
	errs     []error
	calls    int
}

func (c *stubChecker) Name() string {
	return c.name
}

func (c *stubChecker) Check(context.Context) (ProviderBalance, error) {
	idx := c.calls
	c.calls++
	if idx < len(c.errs) && c.errs[idx] != nil {
		return ProviderBalance{}, c.errs[idx]
	}
	if idx >= len(c.balances) {
		idx = len(c.balances) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return c.balances[idx], nil
}

type recordingMessenger struct {
	messages []string
}

func (m *recordingMessenger) SendMessage(_ context.Context, text string) error {
	m.messages = append(m.messages, text)
	return nil
}

func (m *recordingMessenger) last(t *testing.T) string {
	t.Helper()
	if len(m.messages) == 0 {
		t.Fatal("no messages sent")
	}
	return m.messages[len(m.messages)-1]
}

func assertContainsAll(t *testing.T, value string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(value, part) {
			t.Fatalf("message does not contain %q:\n%s", part, value)
		}
	}
}

func mustMoscow(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load Moscow location: %v", err)
	}
	return loc
}
