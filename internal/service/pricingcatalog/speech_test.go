package pricingcatalog

import "testing"

func TestWhisperDEVFixedMinutePrice(t *testing.T) {
	for _, tc := range []struct {
		ms      int64
		seconds int
		credits int64
	}{
		{1, 60, 25}, {60000, 60, 25}, {60001, 120, 50}, {479999, 480, 200}, {480000, 480, 200},
	} {
		seconds, err := WhisperBillableDurationSec(tc.ms)
		if err != nil || seconds != tc.seconds {
			t.Fatalf("duration %d = %d, %v", tc.ms, seconds, err)
		}
		q, err := WhisperDEVQuote(seconds)
		if err != nil || !q.Valid() || q.InternalCredits != tc.credits || q.Key.DurationSec != seconds || q.Source != WhisperDEVFixedPriceSource || q.Floor.Unit != FloorUnitInternalCredits {
			t.Fatalf("fixed DEV quote = %+v, %v", q, err)
		}
	}
	for _, ms := range []int64{-1, 0, 480001, 1 << 62} {
		if _, err := WhisperBillableDurationSec(ms); err == nil {
			t.Fatalf("invalid duration accepted: %d", ms)
		}
	}
	for _, seconds := range []int{-1, 0, 1, 59, 61, 481, 540} {
		if _, err := WhisperDEVQuote(seconds); err == nil {
			t.Fatalf("invalid bucket accepted: %d", seconds)
		}
	}
	catalog, _ := NewStaticCatalog()
	q, _ := WhisperDEVQuote(60)
	if _, err := catalog.Snapshot(q.Key); err == nil {
		t.Fatal("DEV fixed price leaked into static production catalog")
	}
}
