package domain_test

import (
	"math"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestAccountPublicIDCanonicalRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		number int64
		id     string
	}{
		{1, "EP-000001"}, {123, "EP-000123"}, {999999, "EP-999999"},
		{1000000, "EP-1000000"}, {math.MaxInt64, "EP-9223372036854775807"},
	} {
		if got := domain.FormatAccountPublicID(tc.number); got != tc.id {
			t.Fatalf("format %d = %q", tc.number, got)
		}
		for _, value := range []string{tc.id, " " + tc.id + " ", "ep-" + tc.id[3:]} {
			if got, err := domain.ParseAccountPublicID(value); err != nil || got != tc.number {
				t.Fatalf("parse %q = %d/%v", value, got, err)
			}
		}
	}
	for _, number := range []int64{0, -1} {
		if domain.FormatAccountPublicID(number) != "" {
			t.Fatal("invalid number formatted")
		}
	}
}

func TestAccountPublicIDRejectsMalformedAndOutOfRangeInput(t *testing.T) {
	for _, value := range []string{"", "EP-000000", "EP-1", "EP-0000001", "EP--00001", "EP-+00001", "EP-12 345", "EP-123456x", "EP-9223372036854775808", "EP-99999999999999999999", "EP-１２３４５６", "000123", "NH-000123"} {
		if _, err := domain.ParseAccountPublicID(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
