package vk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/adapter/inbound/vk"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

type harness struct {
	handler *vk.Handler
	users   *memory.UserRepo
	cmds    *memory.CommandRepo
	jobs    *memory.JobRepo
	inbound *memory.InboundRepo
	pub     *queue.MemoryPublisher
	relay   *outboxrelay.Relay
}

func newHarness() *harness {
	return newHarnessWithControl(nil)
}

func newHarnessWithControl(control vkdelivery.ControlClient) *harness {
	users := memory.NewUserRepo()
	cmds := memory.NewCommandRepo()
	jobs := memory.NewJobRepo()
	outbox := memory.NewOutboxRepo()
	inbound := memory.NewInboundRepo()
	idem := memory.NewIdempotencyRepo()
	bill := memory.NewBillingRepo()
	billing := billingservice.New(bill)
	pub := queue.NewMemoryPublisher()
	uowMgr := memory.NewUnitOfWork(jobs, outbox, bill)
	orch := joborchestrator.New(jobs, uowMgr, billing, 0)
	h := vk.NewHandler(vk.Config{ConfirmationToken: "conf-token-123", Secret: "s3cr3t"}, vk.Deps{
		Idempotency:  idem,
		Inbound:      inbound,
		Users:        users,
		Commands:     cmds,
		Billing:      billing,
		Orchestrator: orch,
		Router:       commandrouter.New(),
		Control:      control,
	})
	return &harness{handler: h, users: users, cmds: cmds, jobs: jobs, inbound: inbound, pub: pub, relay: outboxrelay.New(uowMgr, pub)}
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

func TestMessageNewStickerCreatesTextJob(t *testing.T) {
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
	if len(cmds) != 1 || !strings.Contains(cmds[0].RawText, "VK-стикер") || !strings.Contains(cmds[0].RawText, "sticker_id=123") {
		t.Fatalf("unexpected command raw text: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 1 || jobs[0].OperationType != domain.OperationTextGenerate {
		t.Fatalf("expected one text job, got %+v", jobs)
	}
	if h.pub.Len() != 1 {
		t.Fatalf("expected 1 enqueued task, got %d", h.pub.Len())
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
	if !strings.Contains(sent[1].Text, "Добро пожаловать в Super GPT") {
		t.Fatalf("unexpected text: %q", sent[1].Text)
	}
	if !strings.Contains(sent[1].Keyboard, `"inline":true`) || !strings.Contains(sent[1].Keyboard, "Создать видео") || !strings.Contains(sent[1].Keyboard, "Пополнить баланс") {
		t.Fatalf("unexpected keyboard: %q", sent[1].Keyboard)
	}
}

func TestShowMenuSendsWelcomeWithoutResettingPersistentKeyboard(t *testing.T) {
	control := vkdelivery.NewMockClient()
	h := newHarnessWithControl(control)
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
	if len(cmds) != 1 || cmds[0].Type != domain.CommandShowMenu {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	jobs, _ := h.jobs.ListByUser(ctx, user.ID, 10, 0)
	if len(jobs) != 0 {
		t.Fatalf("show menu must not create a job, got %d", len(jobs))
	}
	sent := control.Sent()
	if len(sent) != 1 {
		t.Fatalf("expected one welcome message, got %+v", sent)
	}
	if !strings.Contains(sent[0].Text, "Добро пожаловать в Super GPT") {
		t.Fatalf("unexpected text: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Keyboard, `"inline":true`) || strings.Contains(sent[0].Keyboard, "Показать меню") {
		t.Fatalf("unexpected keyboard: %q", sent[0].Keyboard)
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
		"object":{"message":{"from_id":564,"peer_id":564,"text":"💬 Спросить у GPT","payload":"{\"command\":\"menu.text\"}"}}
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
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "SUPER GPT активен") || !strings.Contains(sent[0].Keyboard, "⬅️ Назад") {
		t.Fatalf("unexpected gpt response: %+v", sent)
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
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "Sora 2 выбрана") {
		t.Fatalf("unexpected model response: %+v", sent)
	}
	if !strings.Contains(sent[0].Keyboard, "⬅️ Назад") {
		t.Fatalf("expected back button in keyboard: %q", sent[0].Keyboard)
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
