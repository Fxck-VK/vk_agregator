package legal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestCatalogRequiresRuntimeOperatorAndBindsExactText(t *testing.T) {
	if New("Synthetic Test Operator", "123456789012", true).Checksum != "6c74f01c6d89993e7f4891802d0b04e1c4722f752fd8461e799ea962a8b7e130" {
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

func TestReferralTermsMatchServerPolicy(t *testing.T) {
	p := domain.CurrentWebReferralRewardPolicy()
	if p.Version != 1 || p.Stars != 50 || p.DelayHours != 24 || p.Expires {
		t.Fatal("unreviewed referral policy", p)
	}
	var c sourceCatalog
	if err := json.Unmarshal(source, &c); err != nil {
		t.Fatal(err)
	}
	var text string
	for _, d := range c.Documents {
		if d.Slug == "paid-access" {
			for _, s := range d.Sections {
				if strings.HasPrefix(s.Title, "8 ") {
					text = strings.Join(s.Paragraphs, "\n")
				}
			}
		}
	}
	for _, required := range []string{fmt.Sprintf("%d бонусных звёзд", p.Stars), fmt.Sprintf("%d часа", p.DelayHours), "один раз за каждый", "не ограничено", "не сгорают", "частичном возврате", "отзываются все 50", "ниже нуля", "не вызывает денежного списания", "задним числом не начисляется"} {
		if !strings.Contains(text, required) {
			t.Errorf("missing approved referral rule %q", required)
		}
	}
}

func TestPromotionTermsDescribeFirstPaymentWithoutChangingRenewalOrStars(t *testing.T) {
	var catalog sourceCatalog
	if err := json.Unmarshal(source, &catalog); err != nil {
		t.Fatal(err)
	}
	var terms string
	for _, doc := range catalog.Documents {
		if doc.Slug == "paid-access" {
			for _, section := range doc.Sections {
				if strings.HasPrefix(section.Title, "9 ") {
					terms = strings.Join(section.Paragraphs, "\n")
				}
			}
		}
	}
	for _, required := range []string{"первую оплату тарифа", "без предыдущих успешных покупок", "не меняет количество звёзд", "по обычной цене", "один промокод", "не восстанавливает", "фактически уплаченной суммы"} {
		if !strings.Contains(terms, required) {
			t.Errorf("missing promotion rule %q", required)
		}
	}
}

func TestRetryEditionPreservesPrivacyAndOptionalMarketing(t *testing.T) {
	var catalog sourceCatalog
	if err := json.Unmarshal(source, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.MarketingVersion != "2026-10-23.2" {
		t.Fatal("renewal policy must not rewrite the optional marketing version")
	}
	for slug, expected := range map[string]string{
		"privacy-policy":    "96a6073a73404c1fad5ffcfa6d0cf19972fe84e0c9ba41237e785b1dd12cc121",
		"marketing-consent": "d0f6f4f5862be56a0311229d1503677bc61592554ee260f5108e438437920115",
	} {
		t.Run(slug, func(t *testing.T) {
			var lines []string
			for _, doc := range catalog.Documents {
				if doc.Slug != slug {
					continue
				}
				lines = append(lines, doc.Slug, doc.Title, doc.TitleEN)
				for _, section := range doc.Sections {
					lines = append(lines, section.Title)
					lines = append(lines, section.Paragraphs...)
					for _, row := range section.Table {
						lines = append(lines, row...)
					}
				}
			}
			if digest(strings.Join(lines, "\n")) != expected {
				t.Fatal("renewal policy changed an unrelated reviewed document")
			}
		})
	}
}

func TestRenewalRetryTermsMatchAcceptedCanonicalPolicy(t *testing.T) {
	var policy struct {
		RenewalRetries struct {
			Version       int `json:"version"`
			WindowDays    int `json:"window_days"`
			IntervalHours int `json:"interval_hours"`
			MaxAttempts   int `json:"max_attempts"`
		} `json:"renewal_retries"`
	}
	data, err := os.ReadFile("../../../internal/service/economycatalog/policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	retry := policy.RenewalRetries
	if retry.Version != 1 || retry.WindowDays != 14 || retry.IntervalHours != 24 || retry.MaxAttempts != 14 {
		t.Fatalf("unreviewed renewal retry policy: %+v", retry)
	}
	var catalog sourceCatalog
	if err := json.Unmarshal(source, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Version != "2026-10-31.2" || catalog.Date != "2026-10-31" {
		t.Fatalf("retry terms need the reviewed edition and date: %s / %s", catalog.Version, catalog.Date)
	}
	for _, slug := range []string{"user-agreement", "paid-access"} {
		t.Run(slug, func(t *testing.T) {
			var text string
			for _, doc := range catalog.Documents {
				if doc.Slug == slug {
					for _, section := range doc.Sections {
						text += strings.Join(section.Paragraphs, "\n") + "\n"
					}
				}
			}
			for _, required := range []string{
				"отдельного явного согласия", "недостатка средств", "платёжным поставщиком",
				fmt.Sprintf("не чаще одного раза в %d часа", retry.IntervalHours),
				fmt.Sprintf("в течение %d дней с первой неудачной попытки", retry.WindowDays),
				fmt.Sprintf("не более %d неуспешных попыток, включая первую", retry.MaxAttempts),
				"В момент окончания этого срока и после него новая попытка не начинается",
				"отмены подписки", "отказа от использования способа оплаты", "сразу прекращаются",
				"иной или неизвестной причине", "подтверждения недостатка средств",
				"тариф и цена исходного продления", "после подтверждения успешной оплаты",
				"оплаченный период начинается с момента подтверждения успешной оплаты сервисом",
				fmt.Sprintf("версию %d правил повторных попыток", retry.Version),
				"Ранее данное согласие без этой версии не расширяется",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("missing reviewed renewal disclosure %q", required)
				}
			}
		})
	}
}
