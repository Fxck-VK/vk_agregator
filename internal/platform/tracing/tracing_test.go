package tracing

import (
	"context"
	"errors"
	"testing"
)

func TestSafeErrorClassDoesNotExposeRawErrorText(t *testing.T) {
	raw := errors.New("provider failed with https://private.example/artifact?token=secret and prompt text")

	got := SafeErrorClass(raw)

	if got != "error" {
		t.Fatalf("SafeErrorClass() = %q, want generic error", got)
	}
}

func TestSafeErrorClassKnownErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: "deadline_exceeded"},
		{name: "timeout", err: errors.New("request timeout after 30s"), want: "timeout"},
		{name: "rate_limit", err: errors.New("too many requests"), want: "rate_limit"},
		{name: "auth", err: errors.New("unauthorized"), want: "auth"},
		{name: "invalid", err: errors.New("validation failed"), want: "invalid_input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SafeErrorClass(tt.err); got != tt.want {
				t.Fatalf("SafeErrorClass() = %q, want %q", got, tt.want)
			}
		})
	}
}
