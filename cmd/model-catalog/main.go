// model-catalog exports the static capability inventory without credentials,
// network access or enabling any model. Regenerate the report after registry edits.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/service/providermodels"
)

type entry struct {
	ID           string                            `json:"id"`
	Name         string                            `json:"name"`
	Capabilities *providermodels.ModelCapabilities `json:"capabilities"`
}

func main() {
	format := flag.String("format", "json", "json or markdown")
	flag.Parse()
	r := providermodels.StaticRegistry()
	groups := map[string][]entry{"text": {}, "image": {}, "video": {}, "audio": {}}
	for _, m := range r.TextAliases {
		groups["text"] = append(groups["text"], entry{m.PublicID, m.DisplayName, providermodels.Capabilities(m.PublicID)})
	}
	for _, m := range r.ImageModels {
		groups["image"] = append(groups["image"], entry{m.PublicID, m.DisplayName, providermodels.Capabilities(m.PublicID)})
	}
	for _, m := range r.VideoRouteModels {
		if !m.LoadTestOnly {
			id := string(m.Alias)
			groups["video"] = append(groups["video"], entry{id, providermodels.VideoName(m.Alias), providermodels.Capabilities(id)})
		}
	}
	switch *format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(groups); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "markdown":
		fmt.Print("# Возможности моделей\n\nСгенерировано из backend-реестра командой `go run ./cmd/model-catalog -format markdown`.\n\nЗдесь перечислены все интегрированные модели, включая скрытые флагами. Доступность в конкретном окружении определяется настройками и тарифами. API и реализация приложения указаны отдельно. «Не подтверждено» не означает отсутствие поддержки; пустой список не означает произвольные значения. Возможности API с пометкой о проверенном наборе не претендуют на полный каталог поставщика. Параметры не изменяют текущую тарификацию.\n\nОбщий backend-контракт может быть шире конкретного интерфейса: Mini App создаёт одно изображение 1:1 и принимает фото-референсы; сайт выбирает пропорции и число результатов, но пока не передаёт референсы. В этом файле показан общий backend-контракт. Ответы /miniapp/model-catalog и /web/v1/models (и совместимый /web/v1/image-models) дополнительно сужают capabilities.application под конкретный интерфейс.\n\nИсточники: [данные проверки текстовых API](../internal/service/providermodels/provider_capability_sources.json), [FLUX.2 Pro](https://docs.apimart.ai/ru/api-reference/images/flux-2/generation), [Midjourney Imagine](https://docs.apimart.ai/ru/api-reference/images/midjourney/imagine), [Kling Motion Control](https://docs.apimart.ai/ru/api-reference/videos/kling-v2-6/kling-v2-6-motion-control-generation). Остальные ограничения сверены с реестром и проверками подключённых адаптеров; неполнота данных указана в профиле API.\n\n")
		for _, kind := range []string{"text", "image", "video", "audio"} {
			title := map[string]string{"text": "Текст", "image": "Фото", "video": "Видео", "audio": "Аудио"}[kind]
			fmt.Printf("## %s (%d)\n\n", title, len(groups[kind]))
			if len(groups[kind]) == 0 {
				fmt.Print("Действующих моделей этого типа пока нет. Раздел выделен в структуре backend.\n")
				continue
			}
			for _, e := range groups[kind] {
				fmt.Printf("### %s\n\nID: `%s`\n\n| Параметр | API провайдера | Реализовано в backend |\n|---|---|---|\n", e.Name, e.ID)
				api, app := rows(e.Capabilities.API), rows(e.Capabilities.Application)
				for i, row := range app {
					fmt.Printf("| %s | %s | %s |\n", escape(row[0]), escape(api[i][1]), escape(row[1]))
				}
				fmt.Println()
				for _, note := range e.Capabilities.API.Notes {
					fmt.Printf("API: %s\n\n", note)
				}
				for _, note := range e.Capabilities.Application.Notes {
					fmt.Printf("Приложение: %s\n\n", note)
				}
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "format must be json or markdown")
		os.Exit(2)
	}
}

func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }
func number(n *int) string {
	if n == nil {
		return "Не подтверждено"
	}
	return strconv.Itoa(*n)
}
func list(s []string) string {
	if len(s) == 0 {
		return "Не задано / не подтверждено"
	}
	return strings.Join(s, ", ")
}
func numbers(ns []int) string {
	var out []string
	for _, n := range ns {
		out = append(out, strconv.Itoa(n))
	}
	return list(out)
}
func input(i providermodels.InputCapability) string {
	if i.Support == providermodels.Unsupported {
		return "Нет"
	}
	if i.Support == providermodels.Unknown {
		return "Не подтверждено"
	}
	return "Да; максимум: " + number(i.MaxCount) + "; форматы: " + list(i.Extensions)
}
func rows(p providermodels.CapabilityProfile) [][2]string {
	if c := p.Text; c != nil {
		return [][2]string{{"Принимает фото", input(c.Images)}, {"Принимает видео", input(c.Videos)}, {"Другие файлы: TXT, DOC, PDF и др.", input(c.Files)}}
	}
	if c := p.Image; c != nil {
		return [][2]string{{"Фото на вход", input(c.Images)}, {"Пропорции", list(c.AspectRatios)}, {"Разрешения", list(c.Resolutions)}, {"Детализация", list(c.QualityModes)}, {"Скорость", list(c.SpeedModes)}, {"Максимум результатов", number(c.MaxOutputCount)}, {"Общий лимит фото вход + выход", number(c.MaxCombinedImages)}}
	}
	if c := p.Video; c != nil {
		mode := map[string]string{"selected": "Выбирается", "automatic": "Автоматически", "reference_video": "По исходному видео", "unknown": "Не подтверждено"}[c.Duration.Mode]
		frame := map[string]string{"required": "Обязателен", "optional": "Необязателен", "unsupported": "Не поддерживается", "unknown": "Не подтверждено"}
		audio := map[string]string{"optional": "Со звуком или без", "generated": "Со звуком", "silent": "Без звука", "preserve_source": "Сохранение исходного звука", "unknown": "Не подтверждено"}[c.Audio.Mode]
		byResolution, _ := json.Marshal(c.Duration.ByResolution)
		byOrientation, _ := json.Marshal(c.Duration.ByOrientation)
		return [][2]string{{"Фото на вход", input(c.Images)}, {"Видео на вход", input(c.Videos)}, {"Допустимое число фото", numbers(c.AllowedImageCounts)}, {"Длительность", mode}, {"Минимум, с", number(c.Duration.MinSeconds)}, {"Максимум, с", number(c.Duration.MaxSeconds)}, {"Выбор длительности, с", numbers(c.Duration.AllowedSeconds)}, {"Длительности по разрешению", optionalJSON(byResolution)}, {"Максимум по ориентации", optionalJSON(byOrientation)}, {"Разрешения", list(c.Resolutions)}, {"Режимы качества", list(c.QualityModes)}, {"Пропорции", list(c.AspectRatios)}, {"Звук", audio}, {"Начальный кадр", frame[c.StartFrame]}, {"Конечный кадр", frame[c.EndFrame]}}
	}
	return nil
}

func optionalJSON(value []byte) string {
	if string(value) == "null" {
		return "Не задано"
	}
	return string(value)
}
