package vk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	antispamservice "vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/outboxrelay"
	"vk-ai-aggregator/internal/service/referralservice"
)

type harness struct {
	handler *vk.Handler
	users   *memory.UserRepo
	cmds    *memory.CommandRepo
	jobs    *memory.JobRepo
	inbound *memory.InboundRepo
	billing *memory.BillingRepo
	refs    *memory.ReferralRepo
	pub     *queue.MemoryPublisher
	relay   *outboxrelay.Relay
}

func newHarness() *harness {
	return newHarnessWithControl(nil)
}

func newHarnessWithControl(control vkdelivery.ControlClient) *harness {
	return newHarnessWithConfig(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"})
}

func newHarnessWithConfig(control vkdelivery.ControlClient, cfg vk.Config) *harness {
	return newHarnessWithDeps(control, cfg, nil, nil)
}

func newHarnessWithConfigAndAntiSpam(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam) *harness {
	return newHarnessWithDeps(control, cfg, antiSpam, nil)
}

func newHarnessWithConfigAndDialogState(control vkdelivery.ControlClient, cfg vk.Config, dialogState vk.DialogState) *harness {
	return newHarnessWithDeps(control, cfg, nil, dialogState)
}

func newHarnessWithDeps(control vkdelivery.ControlClient, cfg vk.Config, antiSpam vk.AntiSpam, dialogState vk.DialogState) *harness {
	var profile vkdelivery.UserProfileClient
	if p, ok := control.(vkdelivery.UserProfileClient); ok {
		profile = p
	}
	users := memory.NewUserRepo()
	cmds := memory.NewCommandRepo()
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	inbound := memory.NewInboundRepo()
	idem := memory.NewIdempotencyRepo()
	bill := memory.NewBillingRepo()
	billing := billingservice.New(bill)
	refs := memory.NewReferralRepo()
	referrals := referralservice.New(refs, billing, referralservice.Config{ReferrerSignupRewardCredits: 10})
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox, bill)
	orch := joborchestrator.New(jobs, uowMgr, billing, 0)
	h := vk.NewHandler(cfg, vk.Deps{
		Idempotency:  idem,
		Inbound:      inbound,
		Users:        users,
		Jobs:         jobs,
		Commands:     cmds,
		Billing:      billing,
		Referrals:    referrals,
		Orchestrator: orch,
		Router:       commandrouter.New(),
		Control:      control,
		Profile:      profile,
		DialogState:  dialogState,
		AntiSpam:     antiSpam,
	})
	return &harness{handler: h, users: users, cmds: cmds, jobs: jobs, inbound: inbound, billing: bill, refs: refs, pub: pub, relay: outboxrelay.New(uowMgr, pub)}
}

// post serves the webhook and then drains the outbox relay, mirroring the
// api+worker split where the relay publishes queued jobs to the worker queue.
func (h *harness) post(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/vk", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	_, _ = h.relay.Drain(context.Background())
	return rec
}

func TestConfirmation(t *testing.T) {
	h := newHarness()
	rec := h.post(`{"type":"confirmation","group_id":1,"secret":"s3cr3t"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "conf-token-123" {
		t.Fatalf("body = %q, want confirmation token", got)
	}
}

func TestInvalidSecret(t *testing.T) {
	h := newHarness()
	rec := h.post(`{"type":"confirmation","group_id":1,"secret":"wrong"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestMessageNewCreatesJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-100","secret":"s3cr3t",
		"object":{"message":{"from_id":555,"peer_id":555,"text":"/image neon cat"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 555)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}

	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].OperationType != domain.OperationImageGenerate {
		t.Fatalf("operation = %q, want image_generate", jobs[0].OperationType)
	}
	if h.pub.Len() != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", h.pub.Len())
	}
}

func TestAntiSpamDenialSkipsCommandAndJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	antiSpam := &fakeAntiSpam{
		decision: antispamservice.Decision{
			Allowed: false,
			Kind:    antispamservice.DecisionCooldown,
			Message: "Слишком много сообщений. Попробуйте через 30 секунд",
		},
	}
	h := newHarnessWithConfigAndAntiSpam(control, vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, antiSpam)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-antispam","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"/image neon cat"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(antiSpam.inputs) != 1 || antiSpam.inputs[0].CommandType != domain.CommandImageGenerate {
		t.Fatalf("unexpected anti-spam inputs: %+v", antiSpam.inputs)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 559)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 0 {
		t.Fatalf("anti-spam denial must not create commands, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("anti-spam denial must not create jobs/tasks, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Слишком много сообщений") {
		t.Fatalf("unexpected anti-spam response: %+v", sent)
	}
}

func TestMessageNewStickerOutsideGPTDoesNotCreateTextJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker","secret":"s3cr3t",
		"object":{"message":{
			"from_id":556,"peer_id":556,"text":"",
			"attachments":[{"type":"sticker","sticker":{"sticker_id":123,"product_id":456,"emoji":"🙂"}}]
		}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 556)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("unexpected command raw text: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("sticker outside GPT mode must not create a text job, got %+v", jobs)
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued task, got %d", h.pub.Len())
	}
}

func TestStartSendsWelcomeMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start","secret":"s3cr3t",
		"object":{"message":{"from_id":557,"peer_id":557,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 557)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("start menu must not create a job, got %d", len(jobs))
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if !strings.Contains(sent[0].Keyboard, "Показать меню") || !strings.Contains(sent[0].Keyboard, `"inline":false`) {
		t.Fatalf("unexpected persistent keyboard: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, `"type":"text"`) || strings.Contains(sent[0].Keyboard, `"type":"callback"`) {
		t.Fatalf("persistent keyboard must stay text-only: %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[1].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected text: %q", sent[1].Text)
	}
	if !strings.Contains(sent[1].Keyboard, `"inline":true`) || !strings.Contains(sent[1].Keyboard, "Создать видео") || !strings.Contains(sent[1].Keyboard, "Пополнить баланс") {
		t.Fatalf("unexpected keyboard: %q", sent[1].Keyboard)
	}
	if !strings.Contains(sent[1].Keyboard, `"type":"callback"`) {
		t.Fatalf("inline menu must use callback buttons by default: %q", sent[1].Keyboard)
	}
}

func TestStartWithReferralCodeAppliesSharedReferralNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	ctx := context.Background()
	referrer := &domain.User{
		VKUserID: 900001,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := h.users.Create(ctx, referrer); err != nil {
		t.Fatalf("create referrer: %v", err)
	}
	if err := h.refs.CreateCode(ctx, &domain.ReferralCode{UserID: referrer.ID, Code: "ABC23456"}); err != nil {
		t.Fatalf("create referral code: %v", err)
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-ref","secret":"s3cr3t",
		"object":{"message":{"from_id":900002,"peer_id":900002,"text":"/start ABC23456"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	referred, err := h.users.GetByVKUserID(ctx, 900002)
	if err != nil {
		t.Fatalf("referred user not created: %v", err)
	}
	referral, err := h.refs.GetReferralByReferredUserID(ctx, referred.ID)
	if err != nil {
		t.Fatalf("referral not created: %v", err)
	}
	if referral.ReferrerUserID != referrer.ID || referral.ReferralCode != "ABC23456" || referral.Source != domain.ReferralSourceVKBot {
		t.Fatalf("unexpected referral: %+v", referral)
	}
	if referral.RewardStatus != domain.ReferralRewardApplied {
		t.Fatalf("reward status = %q, want applied", referral.RewardStatus)
	}
	acc, err := h.billing.GetAccountByUser(ctx, referrer.ID, domain.CurrencyCredits)
	if err != nil {
		t.Fatalf("get referrer account: %v", err)
	}
	if acc.BalanceCached != billingservice.DefaultStartingBalance+10 {
		t.Fatalf("referrer balance = %d, want %d", acc.BalanceCached, billingservice.DefaultStartingBalance+10)
	}
	jobs, _ := h.jobs.ListByUser(ctx, referred.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("referral start must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestAccountMenuShowsReferralStatsAndShareLink(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken:                   "conf-token-123",
		Secret:                              "s3cr3t",
		ReferralLinkBase:                    "https://vk.com/write-1",
		ReferralShareBase:                   "https://vk.com/share.php",
		ReferralReferrerSignupRewardCredits: 10,
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-account-ref","secret":"s3cr3t",
		"object":{"message":{"from_id":900003,"peer_id":900003,"text":"Мой аккаунт","payload":"{\"command\":\"account\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected account response, got %+v", sent)
	}
	for _, want := range []string{"Мой аккаунт", "безлимитное общение с НейроХаб", "Реферальная программа", "Приглашённых: 0", "https://vk.com/write-1", "Поддержка: @neirohub_help"} {
		if !strings.Contains(sent[0].Text, want) {
			t.Fatalf("expected %q in account text: %q", want, sent[0].Text)
		}
	}
	for _, notWant := range []string{"Осталось генераций", "Выполнено генераций", "@supergptsupportbot"} {
		if strings.Contains(sent[0].Text, notWant) {
			t.Fatalf("account text should not contain %q: %q", notWant, sent[0].Text)
		}
	}
	if strings.Contains(sent[0].Keyboard, `"type":"open_link"`) || strings.Contains(sent[0].Keyboard, "Поделиться") || strings.Contains(sent[0].Keyboard, "vk.com/share.php") {
		t.Fatalf("account keyboard should not include share button, got %q", sent[0].Keyboard)
	}
	if !strings.Contains(sent[0].Keyboard, "Назад") {
		t.Fatalf("account keyboard should keep back button, got %q", sent[0].Keyboard)
	}
	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 900003)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	code, err := h.refs.GetCodeByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("referral code not created: %v", err)
	}
	if !strings.Contains(sent[0].Text, code.Code) {
		t.Fatalf("account text must include user's referral code %q: %q", code.Code, sent[0].Text)
	}
}

func TestFirstStartUsesVKFirstNameOnce(t *testing.T) {
	control := vkdelivery.NewMockClient()
	control.SetUserProfile(vkdelivery.UserProfile{UserID: 582, FirstName: "Сергей", LastName: "Макаров"})
	h := newHarnessWithControl(control)
	first := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-name-first","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Старт"}}
	}`
	if rec := h.post(first); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected first response: %d %q", rec.Code, rec.Body.String())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[1].Text, "Сергей, добро пожаловать в НейроХаб") {
		t.Fatalf("expected personalized first welcome, got %+v", sent)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 582)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if user.VKFirstName != "Сергей" || user.WelcomeNameSentAt.IsZero() {
		t.Fatalf("expected cached profile and welcome marker, got %+v", user)
	}

	second := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-name-second","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Старт"}}
	}`
	if rec := h.post(second); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected second response: %d %q", rec.Code, rec.Body.String())
	}
	sent = control.Sent()
	if len(sent) != 4 {
		t.Fatalf("expected second persistent keyboard and welcome, got %+v", sent)
	}
	if strings.Contains(sent[3].Text, "Сергей, добро пожаловать") || !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected regular follow-up welcome, got %q", sent[3].Text)
	}
}

func TestMenuFeatureFlagsHideMainMenuButtons(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuStudents: true,
			domain.CommandTopUp:        true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-flags","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if strings.Contains(sent[1].Keyboard, "Студентам и школьникам") || strings.Contains(sent[1].Keyboard, "Пополнить баланс") {
		t.Fatalf("disabled main menu buttons should be hidden: %q", sent[1].Keyboard)
	}
	for _, want := range []string{"Создать видео", "Создать фото", "Спросить у НейроХаб", "Мой аккаунт"} {
		if !strings.Contains(sent[1].Keyboard, want) {
			t.Fatalf("expected enabled button %q in keyboard: %q", want, sent[1].Keyboard)
		}
	}
}

func TestDisabledMenuPayloadFallsBackToCurrentMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuStudents: true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-disabled","secret":"s3cr3t",
		"object":{"message":{"from_id":581,"peer_id":581,"text":"🎁 Студентам и школьникам","payload":"{\"command\":\"menu.students\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 581)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("disabled payload should be recorded as show_menu fallback, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("disabled payload must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("expected current welcome menu fallback, got %+v", sent)
	}
	if strings.Contains(sent[0].Keyboard, "Студентам и школьникам") {
		t.Fatalf("disabled students button should stay hidden in fallback menu: %q", sent[0].Keyboard)
	}
}

func TestTextMenuButtonModeKeepsInlineButtonsAsText(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuButtonMode:    "text",
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-text-mode","secret":"s3cr3t",
		"object":{"message":{"from_id":572,"peer_id":572,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard update and welcome message, got %+v", sent)
	}
	if strings.Contains(sent[1].Keyboard, `"type":"callback"`) || !strings.Contains(sent[1].Keyboard, `"type":"text"`) {
		t.Fatalf("inline menu should use legacy text buttons in text mode: %q", sent[1].Keyboard)
	}
}

func TestShowMenuSendsWelcomeWithoutResettingPersistentKeyboard(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-show-menu-start","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-show-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":559,"peer_id":559,"text":"Показать меню","payload":"{\"command\":\"show_menu\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 559)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandShowMenu) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("show menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 3 {
		t.Fatalf("expected persistent keyboard, start menu, and fresh show-menu message, got %+v", sent)
	}
	if !strings.Contains(sent[2].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected text: %q", sent[2].Text)
	}
	if !strings.Contains(sent[2].Keyboard, `"inline":true`) || strings.Contains(sent[2].Keyboard, "Показать меню") {
		t.Fatalf("unexpected keyboard: %q", sent[2].Keyboard)
	}
	if edits := control.Edits(); len(edits) != 0 {
		t.Fatalf("lower show-menu button must send a fresh menu instead of editing, got edits %+v", edits)
	}
}

func TestTypedMenuRepairPhraseRestoresPersistentKeyboardAndMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-repair-start","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-repair-typed","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"нет меню"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 571)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandShowMenu) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("typed menu repair must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 4 {
		t.Fatalf("expected start keyboard/menu plus repair keyboard/menu, got %+v", sent)
	}
	if !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("typed menu repair should restore lower keyboard, got %+v", sent[2])
	}
	if !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") || !strings.Contains(sent[3].Keyboard, `"inline":true`) {
		t.Fatalf("typed menu repair should send inline welcome menu, got %+v", sent[3])
	}
}

func TestMenuButtonEditsActiveMenuMessage(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-edit-start","secret":"s3cr3t",
		"object":{"message":{"from_id":570,"peer_id":570,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	initial := control.Sent()
	if len(initial) != 2 {
		t.Fatalf("expected persistent keyboard and active menu, got %+v", initial)
	}
	activeID := initial[1].MessageID

	video := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-edit-video","secret":"s3cr3t",
		"object":{"message":{"from_id":570,"peer_id":570,"text":"рџЋ¬ РЎРѕР·РґР°С‚СЊ РІРёРґРµРѕ","payload":"{\"command\":\"menu.video\"}"}}
	}`
	if rec := h.post(video); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected video response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("menu button should edit the active menu instead of sending a new one, got %+v", sent)
	}
	edits := control.Edits()
	if len(edits) != 1 || edits[0].MessageID != activeID {
		t.Fatalf("expected one edit of active menu %d, got %+v", activeID, edits)
	}
	if sent[1].Text != "Выбери модель для генерации:" {
		t.Fatalf("active menu was not updated to video picker: %+v", sent[1])
	}
}

func TestCallbackMenuEventEditsActiveMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-callback-start","secret":"s3cr3t",
		"object":{"message":{"from_id":573,"peer_id":573,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	activeID := control.Sent()[1].MessageID

	callback := `{
		"type":"message_event","group_id":1,"event_id":"evt-callback-video","secret":"s3cr3t",
		"object":{"user_id":573,"peer_id":573,"event_id":"vk-button-event-1","payload":{"command":"menu.video"}}
	}`
	rec := h.post(callback)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected callback response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 573)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandMenuVideo) {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("callback menu event must not create a job, got %d", len(jobs))
	}
	if len(control.Sent()) != 2 {
		t.Fatalf("callback should edit active menu instead of sending a new message, got %+v", control.Sent())
	}
	edits := control.Edits()
	if len(edits) != 1 || edits[0].MessageID != activeID || !strings.Contains(edits[0].Text, "Выбери модель") {
		t.Fatalf("expected active menu edit to video picker, got %+v", edits)
	}
	answers := control.EventAnswers()
	if len(answers) != 1 || answers[0].EventID != "vk-button-event-1" || answers[0].UserID != 573 || answers[0].PeerID != 573 {
		t.Fatalf("expected callback event answer, got %+v", answers)
	}
}

func TestPlainMessageKeepsPreviousMenuAndLowerShowMenuSendsFreshMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-start","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-text","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"РџСЂРёРґСѓРјР°Р№ РёРґРµСЋ РґР»СЏ РІРёРґРµРѕ"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}
	afterPlain := control.Sent()
	if len(afterPlain) != 3 {
		t.Fatalf("plain text should send a choose-mode hint, got %+v", afterPlain)
	}
	if afterPlain[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(afterPlain[2].Keyboard, "Показать меню") || strings.Contains(afterPlain[2].Keyboard, `"inline":true`) {
		t.Fatalf("unexpected choose-mode hint: %+v", afterPlain[2])
	}

	menu := `{
		"type":"message_new","group_id":1,"event_id":"evt-menu-clear-show","secret":"s3cr3t",
		"object":{"message":{"from_id":571,"peer_id":571,"text":"РџРѕРєР°Р·Р°С‚СЊ РјРµРЅСЋ","payload":"{\"command\":\"show_menu\"}"}}
	}`
	if rec := h.post(menu); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected menu response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 4 {
		t.Fatalf("lower show-menu should send a fresh menu below the text hint, got %+v", sent)
	}
	if sent[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("choose-mode hint should keep lower keyboard after lower show-menu, got %+v", sent[2])
	}
	if !strings.Contains(sent[3].Text, "Добро пожаловать в НейроХаб") || !strings.Contains(sent[3].Keyboard, `"inline":true`) {
		t.Fatalf("expected fresh welcome menu after lower show-menu, got %+v", sent[3])
	}
	if edits := control.Edits(); len(edits) != 0 {
		t.Fatalf("lower show-menu must not edit an older menu, got edits %+v", edits)
	}
}

func TestVideoMenuButtonSendsModelPickerNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":560,"peer_id":560,"text":"🎬 Создать видео","payload":"{\"command\":\"menu.video\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 560)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuVideo {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("video menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one model picker message, got %+v", sent)
	}
	if sent[0].Text != "Выбери модель для генерации:" {
		t.Fatalf("unexpected text: %q", sent[0].Text)
	}
	for _, want := range []string{
		"Sora 2 — видео текст+фото",
		"Kling v2.1 — видео текст+фото",
		"Seedance 1 — видео по тексту",
		"Haiuo v0.2 — видео текст+фото",
		"⬅️ Назад",
	} {
		if !strings.Contains(sent[0].Keyboard, want) {
			t.Fatalf("expected %q in keyboard: %q", want, sent[0].Keyboard)
		}
	}
}

func TestPhotoMenuButtonSendsInstructionNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":562,"peer_id":562,"text":"🖼️ Создать фото","payload":"{\"command\":\"menu.image\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 562)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuImage {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("photo menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one photo instruction message, got %+v", sent)
	}
	for _, want := range []string{
		"У вас есть 1 бесплатная попытка",
		"Генерация фото по тексту",
		"Фото по тексту",
		"Фото с референсом",
		"⬅️ Назад",
	} {
		if !strings.Contains(sent[0].Text+sent[0].Keyboard, want) {
			t.Fatalf("expected %q in photo response: text=%q keyboard=%q", want, sent[0].Text, sent[0].Keyboard)
		}
	}
}

func TestPhotoModeButtonIsControlCommandNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-photo-mode","secret":"s3cr3t",
		"object":{"message":{"from_id":563,"peer_id":563,"text":"▶️ Фото по тексту","payload":"{\"command\":\"menu.image.text\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 563)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuImageText {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("photo mode selection must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Генерация фото по тексту выбрана") {
		t.Fatalf("unexpected photo mode response: %+v", sent)
	}
}

func TestGPTMenuButtonSendsActivePromptNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":564,"peer_id":564,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 564)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuText {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("gpt menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "НейроХаб активен") || !strings.Contains(sent[0].Keyboard, "⬅️ Назад") {
		t.Fatalf("unexpected gpt response: %+v", sent)
	}
}

func TestFirstPlainTextStartsOnboardingNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-first-plain-start","secret":"s3cr3t",
		"object":{"message":{"from_id":5740,"peer_id":5740,"text":"привет"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 5740)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandStart {
		t.Fatalf("first plain text should be recorded as onboarding start, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("first onboarding must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected persistent keyboard and welcome, got %+v", sent)
	}
	if !strings.Contains(sent[0].Keyboard, "Показать меню") || !strings.Contains(sent[1].Text, "Добро пожаловать в НейроХаб") {
		t.Fatalf("unexpected onboarding response: %+v", sent)
	}
	if user.WelcomeNameSentAt.IsZero() {
		t.Fatalf("first onboarding should mark welcome as sent: %+v", user)
	}
}

func TestPlainTextOutsideModeRepliesWithChooseModeNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-reply-start","secret":"s3cr3t",
		"object":{"message":{"from_id":574,"peer_id":574,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-reply","secret":"s3cr3t",
		"object":{"message":{"from_id":574,"peer_id":574,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 574)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandUnknown) {
		t.Fatalf("plain text outside mode should be recorded as unknown, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text outside mode must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 3 {
		t.Fatalf("expected one choose-mode response, got %+v", sent)
	}
	if sent[2].Text != "Выберите режим в меню выше или нажмите на кнопку показать меню" || !strings.Contains(sent[2].Keyboard, "Показать меню") || strings.Contains(sent[2].Keyboard, `"inline":true`) {
		t.Fatalf("unexpected choose-mode response: %+v", sent)
	}
}

func TestPlainTextOutsideModeCanBeSilent(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		UnroutedTextMode:  "silent",
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-silent-start","secret":"s3cr3t",
		"object":{"message":{"from_id":575,"peer_id":575,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-silent","secret":"s3cr3t",
		"object":{"message":{"from_id":575,"peer_id":575,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 575)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandUnknown) {
		t.Fatalf("plain text outside mode should be recorded as unknown, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text outside mode must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	if sent := control.Sent(); len(sent) != 2 {
		t.Fatalf("silent mode should not send a choose-mode response, got %+v", sent)
	}
}

func TestPlainTextCanKeepLegacyGPTMode(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		UnroutedTextMode:  "gpt",
	})
	start := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-gpt-start","secret":"s3cr3t",
		"object":{"message":{"from_id":576,"peer_id":576,"text":"/start"}}
	}`
	if rec := h.post(start); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected start response: %d %q", rec.Code, rec.Body.String())
	}
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-unrouted-gpt","secret":"s3cr3t",
		"object":{"message":{"from_id":576,"peer_id":576,"text":"придумай идею"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 576)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandStart, domain.CommandTextAsk) {
		t.Fatalf("legacy gpt mode should record text.ask, got %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("legacy gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestGPTMenuButtonEnablesPlainTextJobs(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-on","secret":"s3cr3t",
		"object":{"message":{"from_id":577,"peer_id":577,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-text","secret":"s3cr3t",
		"object":{"message":{"from_id":577,"peer_id":577,"text":"придумай идею"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 577)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[0].Text, "НейроХаб активен") || sent[1].Text != "НейроХаб думает..." {
		t.Fatalf("unexpected control responses: %+v", sent)
	}
	var params struct {
		Prompt                 string `json:"prompt"`
		VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id"`
	}
	if err := json.Unmarshal(jobs[0].Params, &params); err != nil {
		t.Fatalf("decode job params: %v", err)
	}
	if params.Prompt == "" || params.VKPlaceholderMessageID != sent[1].MessageID {
		t.Fatalf("unexpected job params: %+v, pending message=%+v", params, sent[1])
	}
}

func TestPersistedGPTModeSurvivesHandlerRestart(t *testing.T) {
	dialogState := newFakeDialogState()
	firstControl := vkdelivery.NewMockClient()
	first := newHarnessWithConfigAndDialogState(firstControl, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-persist-on","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"рџ’¬ РЎРїСЂРѕСЃРёС‚СЊ Сѓ РќРµР№СЂРѕРҐР°Р±","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := first.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	if mode, ok := dialogState.modes[590]; !ok || mode != "gpt" {
		t.Fatalf("dialog mode was not persisted: %#v", dialogState.modes)
	}

	// New handler/harness simulates an API process restart: its in-memory mode
	// map is empty, but Redis-backed dialog state is still available.
	secondControl := vkdelivery.NewMockClient()
	second := newHarnessWithConfigAndDialogState(secondControl, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-persist-text","secret":"s3cr3t",
		"object":{"message":{"from_id":590,"peer_id":590,"text":"РїСЂРёРґСѓРјР°Р№ РёРґРµСЋ"}}
	}`
	if rec := second.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := second.users.GetByVKUserID(ctx, 590)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := second.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandTextAsk {
		t.Fatalf("persisted mode should route text to GPT, got %+v", cmds)
	}
	jobs, _ := second.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || second.pub.Len() != 1 {
		t.Fatalf("persisted mode should create one text job, jobs=%+v tasks=%d", jobs, second.pub.Len())
	}
	sent := secondControl.Sent()
	if len(sent) != 1 || sent[0].Text != "НейроХаб думает..." {
		t.Fatalf("unexpected persisted-mode response: %+v", sent)
	}
}

func TestStaleBackCallbackClearsPersistedGPTModeAfterRestart(t *testing.T) {
	dialogState := newFakeDialogState()
	dialogState.modes[591] = "gpt"
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfigAndDialogState(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
	}, dialogState)
	back := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-persist-back","secret":"s3cr3t",
		"object":{"user_id":591,"peer_id":591,"event_id":"evt-gpt-persist-back-inner","payload":{"command":"show_menu"}}
	}`
	if rec := h.post(back); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected back response: %d %q", rec.Code, rec.Body.String())
	}
	if _, ok := dialogState.modes[591]; ok {
		t.Fatalf("stale back callback should clear persisted mode: %#v", dialogState.modes)
	}
	if len(control.EventAnswers()) != 1 {
		t.Fatalf("callback should be acknowledged, got %+v", control.EventAnswers())
	}
	if sent := control.Sent(); len(sent) != 0 {
		t.Fatalf("stale back callback should not send a fresh menu, got %+v", sent)
	}
}

func TestStaleCallbackShowMenuAfterGPTTextDoesNotSendMenu(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-stale-show-on","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}

	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-stale-show-text","secret":"s3cr3t",
		"object":{"message":{"from_id":580,"peer_id":580,"text":"кто такой сантиз?"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	stale := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-stale-show-callback","secret":"s3cr3t",
		"object":{"user_id":580,"peer_id":580,"event_id":"vk-button-event-stale-show","payload":{"command":"show_menu"}}
	}`
	if rec := h.post(stale); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected stale callback response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 580)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk, domain.CommandShowMenu) {
		t.Fatalf("unexpected command types: %+v", commandTypes(cmds))
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[0].Text, "НейроХаб активен") || sent[1].Text != "НейроХаб думает..." {
		t.Fatalf("stale show_menu callback must not send a fresh menu, got %+v", sent)
	}
	answers := control.EventAnswers()
	if len(answers) != 1 || answers[0].EventID != "vk-button-event-stale-show" {
		t.Fatalf("expected stale callback acknowledgement, got %+v", answers)
	}
}

func TestOtherMenuButtonClearsGPTMode(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-clear-on","secret":"s3cr3t",
		"object":{"message":{"from_id":578,"peer_id":578,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	video := `{
		"type":"message_event","group_id":1,"event_id":"evt-gpt-mode-clear-video","secret":"s3cr3t",
		"object":{"user_id":578,"peer_id":578,"event_id":"vk-button-event-clear","payload":{"command":"menu.video"}}
	}`
	if rec := h.post(video); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected video response: %d %q", rec.Code, rec.Body.String())
	}
	plain := `{
		"type":"message_new","group_id":1,"event_id":"evt-gpt-mode-clear-text","secret":"s3cr3t",
		"object":{"message":{"from_id":578,"peer_id":578,"text":"придумай идею"}}
	}`
	if rec := h.post(plain); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected plain response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 578)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandMenuVideo, domain.CommandUnknown) {
		t.Fatalf("unexpected command types after gpt mode clear: %+v", commandTypes(cmds))
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 || h.pub.Len() != 0 {
		t.Fatalf("plain text after mode clear must not create jobs, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
	sent := control.Sent()
	if len(sent) != 2 || !strings.Contains(sent[1].Text, "Выберите режим") {
		t.Fatalf("expected choose-mode response after mode clear, got %+v", sent)
	}
	if !strings.Contains(sent[1].Keyboard, "Показать меню") || strings.Contains(sent[1].Keyboard, `"inline":true`) {
		t.Fatalf("choose-mode response after mode clear must restore only lower keyboard: %+v", sent[1])
	}
	if answers := control.EventAnswers(); len(answers) != 1 || answers[0].EventID != "vk-button-event-clear" {
		t.Fatalf("expected callback event answer, got %+v", answers)
	}
}

func TestStickerInGPTModeCreatesTextJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	gpt := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker-gpt-on","secret":"s3cr3t",
		"object":{"message":{"from_id":579,"peer_id":579,"text":"💬 Спросить у НейроХаб","payload":"{\"command\":\"menu.text\"}"}}
	}`
	if rec := h.post(gpt); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected gpt response: %d %q", rec.Code, rec.Body.String())
	}
	sticker := `{
		"type":"message_new","group_id":1,"event_id":"evt-sticker-gpt-text","secret":"s3cr3t",
		"object":{"message":{
			"from_id":579,"peer_id":579,"text":"",
			"attachments":[{"type":"sticker","sticker":{"sticker_id":123,"product_id":456,"emoji":"🙂"}}]
		}}
	}`
	if rec := h.post(sticker); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected sticker response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 579)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	textCmd, ok := commandByType(cmds, domain.CommandTextAsk)
	if !hasCommandTypes(cmds, domain.CommandMenuText, domain.CommandTextAsk) || !ok || !strings.Contains(textCmd.RawText, "sticker_id=123") {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate || h.pub.Len() != 1 {
		t.Fatalf("sticker in gpt mode should create one text job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

func TestStudentsMenuButtonSendsStudyMenuNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-menu","secret":"s3cr3t",
		"object":{"message":{"from_id":565,"peer_id":565,"text":"🎁 Студентам и школьникам","payload":"{\"command\":\"menu.students\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 565)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuStudents {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("students menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one students menu response, got %+v", sent)
	}
	for _, want := range []string{
		"Данные нейронные сети помогут вам во время учебы",
		"Решальник задач",
		"Генерация презентаций (скоро)",
		"Создание рефератов (скоро)",
		"Ответы на вопросы",
		"⬅️ Назад",
	} {
		if !strings.Contains(sent[0].Text+sent[0].Keyboard, want) {
			t.Fatalf("expected %q in students response: text=%q keyboard=%q", want, sent[0].Text, sent[0].Keyboard)
		}
	}
}

func TestStudentsScenarioButtonIsControlCommandNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-students-solver","secret":"s3cr3t",
		"object":{"message":{"from_id":566,"peer_id":566,"text":"Решальник задач","payload":"{\"command\":\"menu.students.solver\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 566)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuStudentSolver {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("students scenario must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Решальник задач активен") || !strings.Contains(sent[0].Keyboard, "Ответы на вопросы") {
		t.Fatalf("unexpected students scenario response: %+v", sent)
	}
}

func TestVideoModelButtonIsControlCommandNoJob(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-model","secret":"s3cr3t",
		"object":{"message":{"from_id":561,"peer_id":561,"text":"Sora 2 — видео текст+фото","payload":"{\"command\":\"menu.video.sora_2\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 561)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
	if len(cmds) != 1 || cmds[0].Type != domain.CommandMenuVideoSora2 {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("video model selection must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "sora-2") || !strings.Contains(sent[0].Text, "Генерирует видео по тексту или фото") || !strings.Contains(sent[0].Text, "https://t.me/sora_video_1") {
		t.Fatalf("unexpected model response: %+v", sent)
	}
	for _, want := range []string{"😀 Начать генерацию", "ℹ️ Примеры", "⬅️ Назад", "menu.video.sora_2.start", "menu.video.sora_2.examples"} {
		if !strings.Contains(sent[0].Keyboard, want) {
			t.Fatalf("expected %q in keyboard: %q", want, sent[0].Keyboard)
		}
	}
}

func TestNestedMenuFeatureFlagsHideVideoButtons(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithConfig(control, vk.Config{
		ConfirmationToken: "conf-token-123",
		Secret:            "s3cr3t",
		MenuFeatures: vk.MenuFeatureFlags{DisabledCommands: map[domain.CommandType]bool{
			domain.CommandMenuVideoSora2Examples: true,
		}},
	})
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-video-nested-flag","secret":"s3cr3t",
		"object":{"message":{"from_id":582,"peer_id":582,"text":"Sora 2 — видео текст+фото","payload":"{\"command\":\"menu.video.sora_2\"}"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}

	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one Sora response, got %+v", sent)
	}
	if strings.Contains(sent[0].Keyboard, "Примеры") || strings.Contains(sent[0].Keyboard, "menu.video.sora_2.examples") {
		t.Fatalf("disabled nested video button should be hidden: %q", sent[0].Keyboard)
	}
	for _, want := range []string{"😀 Начать генерацию", "⬅️ Назад"} {
		if !strings.Contains(sent[0].Keyboard, want) {
			t.Fatalf("expected enabled nested button %q in keyboard: %q", want, sent[0].Keyboard)
		}
	}
}

func TestVideoNestedButtonsAreControlCommandsNoJob(t *testing.T) {
	tests := []struct {
		name     string
		eventID  string
		text     string
		command  domain.CommandType
		wantText string
		wantKeys []string
	}{
		{
			name:     "sora examples",
			eventID:  "evt-video-sora-examples",
			text:     "ℹ️ Примеры",
			command:  domain.CommandMenuVideoSora2Examples,
			wantText: "Примеры sora-2",
			wantKeys: []string{"⬅️ Назад", "menu.video.sora_2"},
		},
		{
			name:     "sora start",
			eventID:  "evt-video-sora-start",
			text:     "😀 Начать генерацию",
			command:  domain.CommandMenuVideoSora2Start,
			wantText: "Ввод промпта для этой модели",
			wantKeys: []string{"⬅️ Назад", "menu.video.sora_2"},
		},
		{
			name:     "seedance picker",
			eventID:  "evt-video-seedance",
			text:     "Seedance 1 — видео по тексту",
			command:  domain.CommandMenuVideoSeedance1,
			wantText: "Seedance",
			wantKeys: []string{"Seedance 1 Lite", "Seedance 1 Pro", "⬅️ Назад"},
		},
		{
			name:     "seedance lite",
			eventID:  "evt-video-seedance-lite",
			text:     "Seedance 1 Lite",
			command:  domain.CommandMenuVideoSeedance1Lite,
			wantText: "Seedance 1 Lite выбран",
			wantKeys: []string{"⬅️ Назад", "menu.video.seedance_1"},
		},
		{
			name:     "haiuo picker",
			eventID:  "evt-video-haiuo",
			text:     "Haiuo v0.2 — видео текст+фото",
			command:  domain.CommandMenuVideoHaiuo02,
			wantText: "Haiuo 02",
			wantKeys: []string{"Haiuo v0.2 Обычный", "Haiuo v0.2 Fast", "⬅️ Назад"},
		},
		{
			name:     "haiuo fast",
			eventID:  "evt-video-haiuo-fast",
			text:     "Haiuo v0.2 Fast",
			command:  domain.CommandMenuVideoHaiuo02Fast,
			wantText: "Haiuo v0.2 Fast выбран",
			wantKeys: []string{"⬅️ Назад", "menu.video.haiuo_v0_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			control := vkdelivery.NewMockClient()
			h := newHarnessWithControl(control)
			body := fmt.Sprintf(`{
				"type":"message_new","group_id":1,"event_id":%q,"secret":"s3cr3t",
				"object":{"message":{"from_id":561,"peer_id":561,"text":%q,"payload":"{\"command\":\"%s\"}"}}
			}`, tt.eventID, tt.text, tt.command)
			rec := h.post(body)
			if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
				t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
			}

			ctx := context.Background()
			user, err := h.users.GetByVKUserID(ctx, 561)
			if err != nil {
				t.Fatalf("user not created: %v", err)
			}
			cmds, _ := h.cmds.ListByUser(ctx, user.ID, 10, 0)
			if len(cmds) != 1 || cmds[0].Type != tt.command {
				t.Fatalf("unexpected commands: %+v", cmds)
			}
			jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
			if len(jobs) != 0 {
				t.Fatalf("nested video control must not create a job, got %d", len(jobs))
			}
			sent := control.Sent()
			if len(sent) != 1 || !strings.Contains(sent[0].Text, tt.wantText) {
				t.Fatalf("unexpected nested video response: %+v", sent)
			}
			for _, want := range tt.wantKeys {
				if !strings.Contains(sent[0].Keyboard, want) {
					t.Fatalf("expected %q in keyboard: %q", want, sent[0].Keyboard)
				}
			}
		})
	}
}

func TestStartFallsBackWhenVKKeyboardDisabled(t *testing.T) {
	control := &keyboardFailControl{}
	h := newHarnessWithControl(control)
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-start-fallback","secret":"s3cr3t",
		"object":{"message":{"from_id":558,"peer_id":558,"text":"/start"}}
	}`
	rec := h.post(body)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	if len(control.sent) != 3 {
		t.Fatalf("expected keyboard attempt and fallback send, got %+v", control.sent)
	}
	if control.sent[0].Keyboard == nil {
		t.Fatalf("first send must include persistent keyboard")
	}
	if control.sent[1].Keyboard == nil {
		t.Fatalf("second send must include inline keyboard")
	}
	if control.sent[2].Keyboard != nil {
		t.Fatalf("fallback send must omit keyboard")
	}
}

func TestMessageNewDuplicateIsDeduped(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-dup","secret":"s3cr3t",
		"object":{"message":{"from_id":777,"peer_id":777,"text":"/video sunrise"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d", rec.Code)
	}
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("second delivery status = %d", rec.Code)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 777)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 {
		t.Fatalf("expected exactly 1 job after duplicate delivery, got %d", len(jobs))
	}
	if h.pub.Len() != 1 {
		t.Fatalf("expected exactly 1 enqueued task, got %d", h.pub.Len())
	}
}

func TestMessageNewReusesExistingInboundOnRetry(t *testing.T) {
	h := newHarness()
	ctx := context.Background()
	if err := h.inbound.Create(ctx, &domain.InboundEvent{
		Source:         "vk",
		EventType:      "message_new",
		GroupID:        1,
		VKEventID:      "evt-inbound-retry",
		PeerID:         778,
		VKUserID:       778,
		Payload:        json.RawMessage(`{}`),
		Status:         domain.InboundReceived,
		IdempotencyKey: "vk_event:1:evt-inbound-retry",
	}); err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-inbound-retry","secret":"s3cr3t",
		"object":{"message":{"from_id":778,"peer_id":778,"text":"/image retry cat"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("retry delivery status = %d body=%q", rec.Code, rec.Body.String())
	}

	inbound, err := h.inbound.GetByIdempotencyKey(ctx, "vk_event:1:evt-inbound-retry")
	if err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Status != domain.InboundProcessed {
		t.Fatalf("inbound status = %q, want processed", inbound.Status)
	}
	user, err := h.users.GetByVKUserID(ctx, 778)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationImageGenerate || h.pub.Len() != 1 {
		t.Fatalf("retry delivery should create one image job, jobs=%+v tasks=%d", jobs, h.pub.Len())
	}
}

type keyboardFailControl struct {
	sent []vkdelivery.Message
}

func (c *keyboardFailControl) SendMessage(_ context.Context, _ int64, _ int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.sent = append(c.sent, msg)
	if msg.Keyboard != nil {
		return vkdelivery.SendResult{}, &vkdelivery.APIError{Code: 912, Message: "Chat bot feature"}
	}
	return vkdelivery.SendResult{MessageID: int64(len(c.sent)), PeerID: 558}, nil
}

func (c *keyboardFailControl) EditMessage(_ context.Context, _ int64, _ int64, msg vkdelivery.Message) (vkdelivery.SendResult, error) {
	c.sent = append(c.sent, msg)
	if msg.Keyboard != nil {
		return vkdelivery.SendResult{}, &vkdelivery.APIError{Code: 912, Message: "Chat bot feature"}
	}
	return vkdelivery.SendResult{MessageID: int64(len(c.sent)), PeerID: 558}, nil
}

func (c *keyboardFailControl) AnswerMessageEvent(_ context.Context, _ string, _, _ int64) error {
	return nil
}

func commandTypes(cmds []*domain.Command) []domain.CommandType {
	types := make([]domain.CommandType, 0, len(cmds))
	for _, cmd := range cmds {
		types = append(types, cmd.Type)
	}
	return types
}

func hasCommandTypes(cmds []*domain.Command, want ...domain.CommandType) bool {
	if len(cmds) != len(want) {
		return false
	}
	counts := map[domain.CommandType]int{}
	for _, cmd := range cmds {
		counts[cmd.Type]++
	}
	for _, t := range want {
		if counts[t] == 0 {
			return false
		}
		counts[t]--
	}
	return true
}

func commandByType(cmds []*domain.Command, t domain.CommandType) (*domain.Command, bool) {
	for _, cmd := range cmds {
		if cmd.Type == t {
			return cmd, true
		}
	}
	return nil, false
}

type fakeAntiSpam struct {
	decision antispamservice.Decision
	err      error
	inputs   []antispamservice.CheckInput
}

func (f *fakeAntiSpam) Check(_ context.Context, input antispamservice.CheckInput) (antispamservice.Decision, error) {
	f.inputs = append(f.inputs, input)
	return f.decision, f.err
}

type fakeDialogState struct {
	modes map[int64]string
}

func newFakeDialogState() *fakeDialogState {
	return &fakeDialogState{modes: map[int64]string{}}
}

func (f *fakeDialogState) Get(_ context.Context, peerID int64) (string, bool, error) {
	mode, ok := f.modes[peerID]
	return mode, ok, nil
}

func (f *fakeDialogState) Set(_ context.Context, peerID int64, mode string) error {
	f.modes[peerID] = mode
	return nil
}

func (f *fakeDialogState) Clear(_ context.Context, peerID int64) error {
	delete(f.modes, peerID)
	return nil
}

func TestMessageNewControlCommandNoJob(t *testing.T) {
	h := newHarness()
	body := `{
		"type":"message_new","group_id":1,"event_id":"evt-bal","secret":"s3cr3t",
		"object":{"message":{"from_id":888,"peer_id":888,"text":"/balance"}}
	}`
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	ctx := context.Background()
	user, err := h.users.GetByVKUserID(ctx, 888)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("control command must not create a job, got %d", len(jobs))
	}
	if h.pub.Len() != 0 {
		t.Fatalf("expected no enqueued tasks, got %d", h.pub.Len())
	}
}
