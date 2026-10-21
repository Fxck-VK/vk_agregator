package branding

import "testing"

func TestSharedBrandName(t *testing.T) {
	if got, err := parseName(source); err != nil || got != Name() {
		t.Fatalf("embedded name differs from the shared configuration: %v", err)
	}
	for _, name := range []string{"Example Brand", "Пример & Co"} {
		got, err := parseName([]byte(`{"name":"` + name + `","logoPath":"/logo.png"}`))
		if err != nil || got != name {
			t.Fatalf("valid shared brand was not preserved: %v", err)
		}
	}
}

func TestRejectInvalidBrandName(t *testing.T) {
	for _, data := range []string{`{`, `{}`, `{"name":7}`, `{"name":""}`, `{"name":"  "}`, `{"name":" Brand"}`, `{"name":"Brand\r\nBcc: other"}`, `{"name":"Brand\u0000"}`} {
		if _, err := parseName([]byte(data)); err == nil {
			t.Fatal("invalid shared brand configuration accepted")
		}
	}
}
