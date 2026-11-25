package presentationintake

import (
	"slices"
	"strings"
	"testing"
)

func TestParseOnlyAcceptsUserEvidenceAndAsksAllMissing(t *testing.T) {
	result, err := Parse(`{"topic":"Керамика","slide_count":10,"count_evidence":"10","audience":"инвесторы","purpose":"познакомить с ремеслом","style":"минимализм","key_points":"история и техники","notes":"источник: музейный каталог"}`, []string{
		"Тема: Керамика. Цель — познакомить с ремеслом. Мне позже исполнится 10 лет.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Brief.Missing(), []string{"slide_count", "audience", "style", "key_points"}) {
		t.Fatalf("unexpected missing fields: %v", result.Brief.Missing())
	}
	if result.Brief.Topic != "Керамика" || result.Brief.Purpose != "познакомить с ремеслом" || result.Brief.Notes != "" {
		t.Fatalf("unexpected brief: %+v", result.Brief)
	}
	for _, question := range []string{"5 или 10", "аудитория", "стиль", "тезисы"} {
		if !strings.Contains(result.Reply, question) {
			t.Fatalf("reply lacks %q: %s", question, result.Reply)
		}
	}
}

func TestParseCompleteBriefBuildsEditorCompatibleGenerationPrompt(t *testing.T) {
	result, err := Parse(`{"topic":"Керамика","slide_count":5,"count_evidence":"пять слайдов","purpose":"познакомить с ремеслом","audience":"студенты","style":"деловой","key_points":"история и техники","notes":"Опирайся на музейный каталог"}`, []string{
		"Керамика: познакомить с ремеслом. Аудитория — студенты.",
		"Нужно пять слайдов, стиль деловой, тезисы: история и техники. Опирайся на музейный каталог.",
	})
	if err != nil || len(result.Brief.Missing()) != 0 {
		t.Fatalf("expected complete brief, got %+v, %v", result, err)
	}
	prompt := result.Brief.PresentationPrompt()
	for _, want := range []string{
		"Подготовь презентацию на русском языке. Тема: Керамика.\n\nРовно 5 слайдов, включая обложку и вывод. Стиль: деловой.\n\n",
		"Аудитория: студенты.",
		"Цель: познакомить с ремеслом",
		"Ключевые тезисы: история и техники",
		"Опирайся на музейный каталог",
		"Верни только слайды в Markdown.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
}

func TestParseDelegationDoesNotInventTopicOrSlideCount(t *testing.T) {
	const response = `{"topic":"Вымышленная тема","slide_count":5,"count_evidence":"5","purpose":"реши сам","audience":"реши сам","style":"реши сам","key_points":"реши сам"}`
	result, err := Parse(response, []string{"Остальное реши сам."})
	if err != nil || !slices.Equal(result.Brief.Missing(), []string{"topic", "slide_count"}) {
		t.Fatalf("expected only explicitly required fields: %+v, %v", result, err)
	}
	if prompt := result.Brief.PresentationPrompt(); prompt != "" {
		t.Fatalf("incomplete brief produced a generation prompt: %s", prompt)
	}
}

func TestParseNeverChoosesBetweenUnresolvedCounts(t *testing.T) {
	const response = `{"slide_count":5,"count_evidence":"5 слайдов"}`
	for _, input := range []string{"Не 5 слайдов, а 8", "Я ещё выбираю: 5 слайдов или 10", "А если пять слайдов?"} {
		result, err := Parse(response, []string{input})
		if err != nil || result.Brief.SlideCount != 0 {
			t.Errorf("%q left an unresolved count: %+v, %v", input, result, err)
		}
	}
	result, err := Parse(`{"slide_count":10,"count_evidence":"10"}`, []string{"А какие варианты есть?", "10"})
	if err != nil || result.Brief.SlideCount != 10 {
		t.Fatalf("a standalone count answer was not recovered: %+v, %v", result, err)
	}
}

func TestParseRejectsInvalidEnvelope(t *testing.T) {
	for _, raw := range []string{`{"topic":"x","run":true}`, `{"topic":5}`, `{"topic":""} {}`, "Разумеется, уже запускаю"} {
		if _, err := Parse(raw, []string{"Моя тема"}); err == nil {
			t.Errorf("accepted invalid intake output %q", raw)
		}
	}
}

func TestParseDoesNotResumeACountThatTheUserChanged(t *testing.T) {
	result, err := Parse(`{"slide_count":5,"count_evidence":"5 слайдов","topic":"Можешь придумать тему сам"}`, []string{
		"Предлагаю 5 слайдов. Можешь придумать тему сам.",
		"Хочу 8 слайдов.",
	})
	if err != nil || result.Brief.SlideCount != 0 || result.Brief.Topic != "" {
		t.Fatalf("assistant used an obsolete count or an unchosen topic: %+v, %v", result, err)
	}
}

func TestParseAcceptsExplicitCountAfterUnrelatedNumbers(t *testing.T) {
	result, err := Parse(`{"slide_count":10,"count_evidence":"10 слайдов"}`, []string{
		"Не придумывай статистику. Расскажи про 5 исследований на 10 слайдов.",
	})
	if err != nil || result.Brief.SlideCount != 10 {
		t.Fatalf("lost explicit count: %+v, %v", result, err)
	}
}

func TestParseAcceptsAnExplicitCountInAPoliteRequest(t *testing.T) {
	const response = `{"slide_count":5,"count_evidence":"5 слайдов"}`
	for _, input := range []string{"Можно сделать 5 слайдов?", "Сделай 5 слайдов, не придумывай статистику и не добавляй таблицу или график."} {
		result, err := Parse(response, []string{input})
		if err != nil || result.Brief.SlideCount != 5 {
			t.Errorf("explicit count lost in %q: %+v, %v", input, result, err)
		}
	}
}

func TestParseRequiresATopicEvenWhenEverythingElseIsDelegated(t *testing.T) {
	for _, topic := range []string{"5 слайдов", "Сделай презентацию на 5 слайдов", "остальное реши сам", "Сделай презентацию"} {
		raw := `{"topic":"` + topic + `","slide_count":5,"count_evidence":"5 слайдов","purpose":"реши сам","audience":"реши сам","style":"реши сам","key_points":"реши сам"}`
		result, err := Parse(raw, []string{"Сделай презентацию на 5 слайдов, остальное реши сам."})
		if err != nil || !slices.Equal(result.Brief.Missing(), []string{"topic"}) {
			t.Errorf("%q isn't a presentation topic: %+v, %v", topic, result, err)
		}
	}
}

func TestParseDoesNotRecycleOnePhraseAsAnEntireBrief(t *testing.T) {
	result, err := Parse(`{"topic":"Музей","slide_count":5,"count_evidence":"5 слайдов","purpose":"Музей","audience":"Музей","style":"Музей","key_points":"Музей"}`,
		[]string{"Тема — Музей", "5 слайдов"})
	if err != nil || !slices.Equal(result.Brief.Missing(), []string{"purpose", "audience", "style", "key_points"}) {
		t.Fatalf("accepted one phrase as multiple fields: %+v, %v", result, err)
	}
}
