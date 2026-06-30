// Package providertest contains shared assertions for provider adapter tests.
package providertest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

type classifiedError interface {
	ProviderErrorClass() domain.ProviderErrorClass
}

// RequireErrorClass asserts that err carries the expected normalized provider
// error class.
func RequireErrorClass(t testing.TB, err error, want domain.ProviderErrorClass) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", want)
	}
	var classified classifiedError
	if !errors.As(err, &classified) {
		t.Fatalf("error has no provider class: %v", err)
	}
	if got := classified.ProviderErrorClass(); got != want {
		t.Fatalf("error class = %s, want %s; err=%v", got, want, err)
	}
}

// RequireErrorDoesNotContain asserts that an error string does not expose
// sensitive fixtures such as fake provider keys or echoed token values.
func RequireErrorDoesNotContain(t testing.TB, err error, forbidden ...string) {
	t.Helper()
	if err == nil {
		return
	}
	message := err.Error()
	for _, value := range forbidden {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(message, value) {
			t.Fatalf("error leaked %q: %v", value, err)
		}
	}
}

// RequireCapability asserts that the advertised capabilities include one
// operation/modality/model tuple.
func RequireCapability(t testing.TB, caps []domain.Capability, op domain.OperationType, mod domain.Modality, model string) {
	t.Helper()
	for _, cap := range caps {
		if cap.Operation == op && cap.Modality == mod && cap.ModelCode == model {
			return
		}
	}
	t.Fatalf("capability %s/%s/%s not found in %+v", op, mod, model, caps)
}

// RequireRawNotContains asserts that sanitized provider metadata does not
// persist private provider URLs, query tokens or raw provider error text.
func RequireRawNotContains(t testing.TB, raw json.RawMessage, forbidden ...string) {
	t.Helper()
	data := string(raw)
	for _, value := range forbidden {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(data, value) {
			t.Fatalf("raw metadata leaked %q: %s", value, data)
		}
	}
}
