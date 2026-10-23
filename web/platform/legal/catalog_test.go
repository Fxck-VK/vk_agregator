package legal

import (
	"strings"
	"testing"
)

func TestCatalogRequiresRuntimeOperatorAndBindsExactText(t *testing.T) {
	if New("Synthetic Test Operator", "123456789012", true).Checksum != "48fae60b1488ad443a49f80d2376c094847ef6a36ff472921477fa24ccf35da1" {
		t.Fatal("Go/Node legal catalogue parity fixture changed; review both runtimes")
	}
	if New("", "", true).Published {
		t.Fatal("missing operator published")
	}
	first := New("Тестовый Оператор Сервиса", "123456789012", true)
	other := New("Другой Тестовый Оператор", "123456789012", true)
	if !first.Published || first.Checksum == other.Checksum {
		t.Fatal("operator must be bound to content")
	}
	if strings.Contains(string(first.Snapshot), "{NAME}") || strings.Contains(string(first.Snapshot), "{INN}") {
		t.Fatal("unresolved operator")
	}
	if first.Checksum == New("Тестовый Оператор Сервиса", "123456789012", false).Checksum {
		t.Fatal("publication state must bind snapshot")
	}
	if len(first.Documents) != 5 {
		t.Fatal("missing separate consent")
	}
	if err := ValidateSource(); err != nil {
		t.Fatal(err)
	}
}
