package providermodels

import (
	"slices"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
)

const videoExpansionCheckedAt = "2026-09-30"

// Only the generation pages selected by the user establish these API facts.
// Native formats/limits that those pages do not specify remain unknown.
func videoExpansionCandidates() []MediaCandidate {
	out := []MediaCandidate{}
	for _, s := range []struct {
		id, name, native, page   string
		min, max, images, videos int
		res, ratios, formats     []string
		audio, note              string
	}{
		{"flux_3_video", "FLUX 3 Video", "flux-3-video", "flux-3-video/generation", 5, 20, 10, 1, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "2:1"}, nil, videoAudioOptional, "API использует hd/fhd; 720p/1080p — допустимые алиасы. До 10 упорядоченных кадров без временных меток. Продолжение видео и черновик/финализация имеют отдельные тарифы и пока не подключены. Форматы и размер изображений на странице не перечислены."},
		{"pixverse_v6", "PixVerse v6", "pixverse-v6", "pixverse-v6/generation", 1, 15, 7, 0, []string{"360p", "540p", "720p", "1080p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "2:3", "3:2", "21:9"}, nil, videoAudioOptional, "Промпт до 5000 символов. Переход между первым/последним кадрами — только 5 или 8 секунд. До 7 референсов; при обычном image-to-video используется одно фото. Продление по ID задачи пока не подключено. Аудио оплачивается отдельно."},
		{"vidu_q3", "Vidu Q3 Standard", "viduq3", "vidu-q3/generation", 3, 16, 7, 0, []string{"540p", "720p", "1080p"}, []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, []string{"jpg", "jpeg", "png", "webp"}, videoAudioUnknown, "Обязательны 1–7 референсов, до 50 МБ каждый, минимум 128×128, пропорции от 1:4 до 4:1. Промпт до 5000 символов. Управление звуком не описано."},
		{"vidu_q3_mix", "Vidu Q3 Mix", "viduq3-mix", "vidu-q3/generation", 1, 16, 7, 0, []string{"720p", "1080p"}, []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, []string{"jpg", "jpeg", "png", "webp"}, videoAudioUnknown, "Обязательны 1–7 референсов, до 50 МБ каждый, минимум 128×128, пропорции от 1:4 до 4:1. Промпт до 5000 символов. Разрешение 540p не поддерживается; управление звуком не описано."},
		{"vidu_q3_turbo", "Vidu Q3 Turbo", "viduq3-turbo", "vidu-q3-pro/generation", 1, 16, 2, 0, []string{"540p", "720p", "1080p"}, []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, nil, videoAudioOptional, "Промпт до 2000 символов, необязателен с кадрами. При 1–2 изображениях aspect_ratio запрещён. Seed от -1 до 4294967295. Форматы и размер входных изображений на странице не перечислены."},
		{"kling_video_o1", "Kling Video O1", "kling-video-o1", "kling-video-o1/generation", 5, 10, 2, 1, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}, nil, videoAudioUnknown, "Режимы std=720P и pro=1080P; длительность только 5/10 секунд, промпт до 2500 символов. image_urls — до 2 кадров; лимит изображений с ролями reference не указан. Редактирование и видеореференс video_list пока не подключены."},
		{"minimax_h3_max", "MiniMax H3 Max", "MiniMax-H3-Max", "minimax-h3/max", 5, 15, 9, 3, []string{"480p", "768p", "1080p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, []string{"jpg", "jpeg", "png", "webp", "heic", "heif"}, videoAudioGenerated, "Промпт до 7000 символов. Кадры и референсы несовместимы. Фото до 30 МБ, стороны 256–5760, пропорции 0.4–2.5. Первые 2 фото бесплатны, остальные оплачиваются. В 1080P нельзя передавать watermark даже false. Видеореференсы и аудио пока не подключены."},
		{"wan_3_0_prime", "Wan 3.0 Prime", "wan3.0-video-prime", "wan3.0-video/generation", 2, 30, 10, 5, []string{"480p", "720p", "1080p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4"}, []string{"jpg", "jpeg", "png", "bmp", "webp"}, videoAudioOptional, "Тот же API-контракт, что у Wan 3.0. Промпт до 20000 символов. Кадры и референсы несовместимы. Изображения до 20 МБ, стороны 240–8000, отношение сторон до 8:1; PNG без альфы. Видеореференсы, аудио, документы, страницы и автоматическая длительность пока не подключены."},
		{"wan_2_7", "Wan 2.7", "wan2.7", "wan2.7/generation", 2, 15, 2, 1, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4"}, nil, videoAudioUnknown, "Промпт до 5000 символов, негативный — до 500. В документации audio_url одновременно запрещён вместе с изображениями и показан с ними в примере: этот сценарий выключен. Продление видео и входное аудио пока не подключены. Форматы и размер изображений не перечислены."},
		{"gemini_omni_flash_preview", "Gemini Omni Flash Preview", "gemini-omni-flash-preview", "gemini-omni-flash-preview/generation", 3, 10, 16, 1, []string{"720p"}, []string{"16:9", "9:16"}, []string{"jpg", "jpeg", "png"}, videoAudioGenerated, "Автоматическая длительность 3–10 секунд; duration и audio не являются параметрами запроса. Выход 720p/24fps со звуком. До 16 фото. Редактирование по видео и продолжение по ID задачи пока не подключены."},
	} {
		api := VideoCapabilities{Images: inputCapability(Supported, integer(s.images), s.formats...), Videos: noInput(), AllowedImageCounts: intSequence(0, s.images), Duration: DurationCapability{Mode: videoDurationSelected, MinSeconds: integer(s.min), MaxSeconds: integer(s.max), AllowedSeconds: intSequence(s.min, s.max)}, Resolutions: s.res, AspectRatios: s.ratios, Audio: VideoAudioCapability{Mode: s.audio, Selectable: s.audio == videoAudioOptional}, StartFrame: frameOptional, EndFrame: frameOptional}
		if s.videos > 0 {
			api.Videos = inputCapability(Supported, integer(s.videos))
		}
		if s.id == "kling_video_o1" {
			api.Duration.AllowedSeconds = []int{5, 10}
			api.QualityModes = []string{"std", "pro"}
			api.Images.MaxCount = nil
			api.AllowedImageCounts = nil
		}
		if VideoCandidateRequiresImages(s.id) {
			api.AllowedImageCounts = intSequence(1, 7)
			api.StartFrame = frameUnsupported
			api.EndFrame = frameUnsupported
		}
		if s.id == "gemini_omni_flash_preview" {
			api.Duration.Mode = videoDurationAutomatic
			api.Duration.AllowedSeconds = nil
			api.StartFrame = frameUnsupported
			api.EndFrame = frameUnsupported
		}
		app := api
		app.Images = noInput()
		app.Videos = noInput()
		app.AllowedImageCounts = []int{0}
		app.StartFrame = frameUnsupported
		app.EndFrame = frameUnsupported
		if app.Audio.Selectable {
			app.Audio = VideoAudioCapability{Mode: videoAudioSilent}
		}
		appNotes := []string{"Модель ещё не проверена в приложении. Запуск и вложения пока недоступны."}
		if VideoCandidateRequiresImages(s.id) {
			appNotes = append(appNotes, "Генерация без референсов недопустима; до подключения загрузки модель остаётся выключенной.")
		}
		if s.id == "flux_3_video" || s.id == "gemini_omni_flash_preview" {
			appNotes = append(appNotes, "Локальный предел промпта — 20000 символов; предел провайдера на странице не указан.")
		}
		out = append(out, MediaCandidate{PublicID: s.id, Name: s.name, Kind: "video", Provider: domain.ProviderAPIMart, ModelCode: s.native, Documentation: apimartDocsBase + "videos/" + s.page, CheckedAt: videoExpansionCheckedAt, Capabilities: ModelCapabilities{SchemaVersion: 1, API: CapabilityProfile{Video: &api, Notes: []string{s.note}}, Application: CapabilityProfile{Video: &app, Notes: appNotes}}})
	}
	return out
}

func VideoCandidateRequiresImages(id string) bool { return id == "vidu_q3" || id == "vidu_q3_mix" }

func IsVideoExpansion(id string) bool { _, ok := videoExpansionCandidateByID(id); return ok }

func videoExpansionCandidateByID(id string) (MediaCandidate, bool) {
	for _, c := range videoExpansionCandidates() {
		if c.PublicID == id {
			return c, true
		}
	}
	return MediaCandidate{}, false
}

func VideoCandidateDefaults(id string) (string, int) {
	switch id {
	case "pixverse_v6":
		return "540p", 5
	case "minimax_h3_max":
		return "768p", 5
	case "wan_3_0_prime", "wan_2_7":
		return "1080p", 5
	case "gemini_omni_flash_preview":
		return "720p", 10
	default:
		return "720p", 5
	}
}

func videoExpansionFacts(c MediaCandidate) []MediaCandidateOperationFact {
	ids := []string{"text_to_video", "frame_video"}
	switch c.PublicID {
	case "vidu_q3", "vidu_q3_mix":
		ids = []string{"reference_image_to_video"}
	case "pixverse_v6":
		ids = []string{"text_to_video", "image_to_video", "first_last_frame", "reference_image_to_video"}
	case "vidu_q3_turbo":
		ids = []string{"text_to_video", "image_to_video", "first_last_frame"}
	case "minimax_h3_max", "wan_3_0_prime":
		ids = append(ids, "reference_image_to_video")
	case "gemini_omni_flash_preview":
		ids = []string{"text_to_video", "reference_image_to_video"}
	}
	out := []MediaCandidateOperationFact{}
	for _, id := range ids {
		inputs := []string{"prompt"}
		if id == "frame_video" || id == "image_to_video" || id == "first_last_frame" {
			inputs = append(inputs, "first/last frames")
		}
		if id == "reference_image_to_video" {
			inputs = append(inputs, "reference images")
		}
		out = append(out, MediaCandidateOperationFact{ID: id, Kind: "video", Endpoint: apimartVideoEndpoint, NativeVersion: c.ModelCode, SourceID: c.PublicID + "_generation", InputModes: inputs, KnownLimits: append([]string(nil), c.Capabilities.API.Notes...)})
	}
	return out
}

func videoExpansionDraftOperation(c MediaCandidate, fact MediaCandidateOperationFact) modelcontract.Operation {
	api := c.Capabilities.API.Video
	inputs := explicitUnsupportedInputs()
	start, end := frameUnsupported, frameUnsupported
	ratios := api.AspectRatios
	durations := api.Duration.AllowedSeconds
	if api.Duration.Mode == videoDurationAutomatic {
		durations = intSequence(*api.Duration.MinSeconds, *api.Duration.MaxSeconds)
	}
	if fact.ID != "text_to_video" {
		count := 2
		if fact.ID == "image_to_video" {
			count = 1
		}
		if fact.ID == "reference_image_to_video" {
			count = *api.Images.MaxCount
		}
		if c.PublicID == "flux_3_video" {
			count = 10
		}
		formats := []modelcontract.FileFormat{}
		for _, ext := range api.Images.Extensions {
			mime := map[string]string{"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp", "bmp": "image/bmp", "heic": "image/heic", "heif": "image/heif"}[ext]
			formats = append(formats, modelcontract.FileFormat{Extension: "." + ext, MIME: mime})
		}
		inputs.Images = modelcontract.Input{Support: modelcontract.Supported, Enabled: false, Required: true, Processing: "native", MaxCount: count, AllowedCounts: intSequence(1, count), Formats: formats}
		if fact.ID == "first_last_frame" {
			inputs.Images.AllowedCounts = []int{2}
			start = frameRequired
			end = frameRequired
		}
		if fact.ID == "image_to_video" {
			start = frameRequired
		}
		if c.PublicID == "pixverse_v6" && fact.ID == "first_last_frame" {
			durations = []int{5, 8}
		}
		switch c.PublicID {
		case "vidu_q3", "vidu_q3_mix":
			inputs.Images.MaxBytes = 50 << 20
		case "minimax_h3_max":
			inputs.Images.MaxBytes = 30 << 20
			inputs.Images.MaxWidth = 5760
			inputs.Images.MaxHeight = 5760
		case "wan_3_0_prime":
			inputs.Images.MaxBytes = 20 << 20
			inputs.Images.MaxWidth = 8000
			inputs.Images.MaxHeight = 8000
		}
		if fact.ID == "frame_video" {
			start = frameOptional
			end = frameOptional
		}
		if c.PublicID == "vidu_q3_turbo" {
			ratios = []string{""}
			start = frameRequired
		}
	}
	variants := []modelcontract.VideoVariant{}
	audios := []bool{false}
	if api.Audio.Mode == videoAudioGenerated {
		audios = []bool{true}
	} else if api.Audio.Selectable {
		audios = append(audios, true)
	}
	for _, res := range api.Resolutions {
		for _, sec := range durations {
			for _, ratio := range ratios {
				for _, audio := range audios {
					fps := 0
					if c.PublicID == "gemini_omni_flash_preview" {
						fps = 24
					}
					variants = append(variants, modelcontract.VideoVariant{Resolution: res, DurationSec: sec, AspectRatio: ratio, Audio: audio, FPS: fps})
				}
			}
		}
	}
	return modelcontract.Operation{ID: fact.ID, Kind: "video", Inputs: inputs, Video: &modelcontract.VideoOutput{Variants: variants, StartImage: start, EndImage: end}}
}

// Catalog callers can show researched limits even when no safe quote exists.
func VideoCandidateDurations(c MediaCandidate) []int {
	v := c.Capabilities.API.Video
	if v == nil {
		return nil
	}
	if v.Duration.Mode == videoDurationAutomatic {
		return []int{*v.Duration.MaxSeconds}
	}
	return slices.Clone(v.Duration.AllowedSeconds)
}
