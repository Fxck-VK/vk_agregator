package domain

import "testing"

func TestPromotionCodeAndIntegerAmount(t *testing.T) {
	for _, s := range []string{"abc", " Ab_c-123 "} {
		if _, err := NormalizePromotionCode(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"ab", "промо", "a b", "ＡBC"} {
		if _, err := NormalizePromotionCode(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	if got, _ := NormalizePromotionCode(" abc "); got != "ABC" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		base    int64
		percent int
		want    int64
	}{{19900, 15, 16915}, {101, 1, 100}, {1, 99, 1}, {9223372036854775807, 99, 92233720368547759}} {
		got, e := PromotionAmount(tc.base, tc.percent)
		if e != nil || got != tc.want {
			t.Fatalf("amount=%d error=%v want=%d", got, e, tc.want)
		}
	}
	for _, p := range []int{0, 100, -1} {
		if _, e := PromotionAmount(19900, p); e == nil {
			t.Fatal("invalid percent")
		}
	}
}
