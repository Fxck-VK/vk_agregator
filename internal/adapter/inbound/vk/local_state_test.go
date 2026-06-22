package vk

import (
	"context"
	"testing"
	"time"
)

func TestLocalUIStateEvictsExpiredEntries(t *testing.T) {
	h := NewHandler(Config{
		LocalUIStateTTL:        time.Nanosecond,
		LocalUIStateMaxEntries: 4,
	}, Deps{})

	h.setActiveMenu(1, 101)
	h.setDialogMode(context.Background(), 1, dialogModeGPT)
	time.Sleep(time.Millisecond)

	if _, ok := h.getActiveMenu(1); ok {
		t.Fatal("expired active menu should be evicted")
	}
	if _, ok := h.getDialogMode(context.Background(), 1); ok {
		t.Fatal("expired local dialog mode should be evicted without shared state")
	}
	if got := len(h.activeMenus); got != 0 {
		t.Fatalf("expired active menu remained in map: %d", got)
	}
	if got := len(h.dialogModes); got != 0 {
		t.Fatalf("expired dialog mode remained in map: %d", got)
	}
}

func TestLocalUIStateCapsEntries(t *testing.T) {
	h := NewHandler(Config{
		LocalUIStateTTL:        time.Hour,
		LocalUIStateMaxEntries: 2,
	}, Deps{})

	for peerID := int64(1); peerID <= 5; peerID++ {
		h.setActiveMenu(peerID, peerID+100)
		h.setDialogMode(context.Background(), peerID, dialogModeGPT)
	}

	if got := len(h.activeMenus); got > 2 {
		t.Fatalf("active menu map exceeded cap: %d", got)
	}
	if got := len(h.dialogModes); got > 2 {
		t.Fatalf("dialog mode map exceeded cap: %d", got)
	}
	if _, ok := h.getActiveMenu(5); !ok {
		t.Fatal("newest active menu entry should be kept when trimming")
	}
	if _, ok := h.getDialogMode(context.Background(), 5); !ok {
		t.Fatal("newest dialog mode entry should be kept when trimming")
	}
}
