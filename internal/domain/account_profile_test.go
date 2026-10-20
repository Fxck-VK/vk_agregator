package domain

import (
	"strings"
	"testing"
)

func TestNormalizeAccountDisplayName(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"unicode", "  Тестовый\t Пользователь\n", "Тестовый Пользователь"},
		{"formatting", "Test\u202eName\x00", "Test Name"},
		{"missing", " \t\n", ""},
		{"maximum", strings.Repeat("Я", 160), strings.Repeat("Я", 160)},
		{"too long", strings.Repeat("Я", 161), ""},
		{"invalid utf8", "bad\xffname", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeAccountDisplayName(tc.input); got != tc.want {
				t.Fatal("unexpected normalized display name")
			}
		})
	}
}
