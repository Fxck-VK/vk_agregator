package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
)

func TestPresentationHistoryPostgresPaginationAndOwnership(t *testing.T) {
	ctx := context.Background()
	pool := conversationManagementIntegrationPool(t, ctx)
	if _, err := pool.Exec(ctx, `
 ALTER TABLE jobs ADD COLUMN channel TEXT, ADD COLUMN result_mode TEXT, ADD COLUMN operation_type TEXT,
 ADD COLUMN modality TEXT, ADD COLUMN idempotency_key TEXT, ADD COLUMN user_id UUID, ADD COLUMN vk_peer_id BIGINT,
 ADD COLUMN command_id UUID, ADD COLUMN recipient_ref TEXT, ADD COLUMN thread_ref TEXT, ADD COLUMN target_channel TEXT,
 ADD COLUMN target_recipient_ref TEXT, ADD COLUMN target_thread_ref TEXT, ADD COLUMN error_code TEXT;
 ALTER TABLE conversation_messages ADD COLUMN job_id UUID, ADD COLUMN role TEXT,
 ADD COLUMN deleted_at TIMESTAMPTZ, ADD COLUMN redacted_at TIMESTAMPTZ;`); err != nil {
		t.Fatalf("extend isolated fixtures: %v", err)
	}
	owner, otherOwner := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (id) VALUES ($1), ($2)`, owner, otherOwner); err != nil {
		t.Fatal(err)
	}
	conversation := func(account uuid.UUID, source, status string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, account_id, source, status, title, title_origin)
 VALUES ($1, $2, $3, $4, '## Legacy headline', 'auto_generated')`, id, account, source, status); err != nil {
			t.Fatal(err)
		}
		return id
	}
	active, archived := conversation(owner, "web", "active"), conversation(owner, "web", "archived")
	foreign, mini := conversation(otherOwner, "web", "active"), conversation(owner, "miniapp", "active")
	at := time.Date(2026, 11, 25, 13, 39, 10, 0, time.UTC)
	job := func(id uuid.UUID, account uuid.UUID, thread, key string, offset time.Duration) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO jobs (id,account_id,source,status,params,created_at,channel,result_mode,operation_type,modality,idempotency_key)
 VALUES ($1,$2,'web','failed_terminal',jsonb_build_object('conversation_id',$3::text,'conversation_source','web','model_name','Qwen'),$4,
 'web','account_history','text_generate','text',$5)`, id, account, thread, at.Add(offset), key); err != nil {
			t.Fatal(err)
		}
	}
	key := func(account uuid.UUID) string { return domain.WebPresentationJobKeyPrefix(account) + uuid.NewString() }
	newest := uuid.MustParse("10000000-0000-4000-8000-000000000002")
	older := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	job(newest, owner, active.String(), key(owner), 0)
	job(older, owner, active.String(), key(owner), 0)
	job(uuid.New(), owner, archived.String(), key(owner), time.Minute)
	job(uuid.New(), owner, foreign.String(), key(owner), time.Minute)
	job(uuid.New(), owner, mini.String(), key(owner), time.Minute)
	job(uuid.New(), otherOwner, foreign.String(), key(otherOwner), time.Minute)
	job(uuid.New(), owner, active.String(), key(otherOwner), time.Minute)
	job(uuid.New(), owner, active.String(), "web-chat:"+owner.String()+":"+uuid.NewString(), time.Minute)
	job(uuid.New(), owner, active.String(), domain.WebPresentationJobKeyPrefix(owner)+uuid.Nil.String(), time.Minute)
	job(uuid.New(), owner, "invalid conversation id", key(owner), time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO conversation_messages (id, conversation_id, job_id, role, text, redacted_at)
 VALUES ($1,$2,$3,'user','First history topic',NULL),($4,$2,$5,'user','Already redacted topic',now()),
 ($6,$2,$3,'assistant','Never include answer text',NULL)`, uuid.New(), active, newest, uuid.New(), older, uuid.New()); err != nil {
		t.Fatal(err)
	}
	repository := postgres.NewPresentationHistoryRepository(pool)
	first, err := repository.ListPresentationHistory(ctx, owner, 1, nil)
	if err != nil || len(first) != 1 || first[0].JobID != newest || first[0].ConversationID != active || first[0].TitlePrompt != "First history topic" {
		t.Fatalf("first owned presentation page: %#v, %v", first, err)
	}
	next, err := repository.ListPresentationHistory(ctx, owner, 2, &domain.JobCursor{ID: newest, CreatedAt: at})
	if err != nil || len(next) != 1 || next[0].JobID != older || next[0].TitlePrompt != "" {
		t.Fatalf("second owned presentation page: %#v, %v", next, err)
	}
}
