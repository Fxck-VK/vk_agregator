package vkdelivery_test

import (
	"context"
	"errors"
	"testing"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
)

func TestDeterministicRandomIDStableAndNonNegative(t *testing.T) {
	a := vkdelivery.DeterministicRandomID("delivery:job-1")
	b := vkdelivery.DeterministicRandomID("delivery:job-1")
	c := vkdelivery.DeterministicRandomID("delivery:job-2")
	if a != b {
		t.Fatalf("same key must give same id: %d != %d", a, b)
	}
	if a == c {
		t.Fatalf("different keys should differ: %d == %d", a, c)
	}
	if a < 0 {
		t.Fatalf("random id must be non-negative, got %d", a)
	}
}

func TestMockSendAndDedup(t *testing.T) {
	c := vkdelivery.NewMockClient()
	ctx := context.Background()
	rid := vkdelivery.DeterministicRandomID("delivery:job-1")

	res, err := c.SendText(ctx, 100, rid, "hello")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.MessageID == 0 || res.Duplicate {
		t.Fatalf("unexpected first result: %+v", res)
	}

	// Same random_id -> deduped, no second message.
	dup, err := c.SendText(ctx, 100, rid, "hello")
	if err != nil {
		t.Fatalf("dup send: %v", err)
	}
	if !dup.Duplicate || dup.MessageID != res.MessageID {
		t.Fatalf("expected dedup with same message id, got %+v", dup)
	}
	if len(c.Sent()) != 1 {
		t.Fatalf("expected exactly one recorded send, got %d", len(c.Sent()))
	}
}

func TestMockSendPhotoVideo(t *testing.T) {
	c := vkdelivery.NewMockClient()
	ctx := context.Background()
	if _, err := c.SendPhoto(ctx, 1, 11, "photo123_456", "cap"); err != nil {
		t.Fatalf("photo: %v", err)
	}
	if _, err := c.SendVideo(ctx, 1, 22, "video123_456", ""); err != nil {
		t.Fatalf("video: %v", err)
	}
	sent := c.Sent()
	if len(sent) != 2 || sent[0].Type != "photo" || sent[1].Type != "video" {
		t.Fatalf("unexpected sends: %+v", sent)
	}
}

func TestMockFailNext(t *testing.T) {
	c := vkdelivery.NewMockClient()
	want := errors.New("boom")
	c.FailNext(want)
	if _, err := c.SendText(context.Background(), 1, 1, "x"); !errors.Is(err, want) {
		t.Fatalf("expected injected error, got %v", err)
	}
	// Subsequent sends succeed.
	if _, err := c.SendText(context.Background(), 1, 2, "y"); err != nil {
		t.Fatalf("expected success after failure, got %v", err)
	}
}
