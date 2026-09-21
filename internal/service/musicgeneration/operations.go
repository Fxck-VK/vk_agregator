package musicgeneration

import "vk-ai-aggregator/internal/domain"

type Operation struct {
	ID                  domain.MusicAction `json:"-"`
	Title               string             `json:"title"`
	Group               string             `json:"group"`
	OutputKind          string             `json:"output_kind"`
	SupportsMax         bool               `json:"supports_max"`
	SupportsCustomModel bool               `json:"supports_custom_model"`
	SupportsPersona     bool               `json:"supports_persona"`
	SupportsAudioFormat bool               `json:"supports_audio_format"`
	MinSources          int                `json:"min_sources"`
	MaxSources          int                `json:"max_sources"`
	MinUploads          int                `json:"min_uploads"`
	MaxUploads          int                `json:"max_uploads"`
}

// Operations lists current non-deprecated actions. Native API methods are an
// adapter concern; these IDs are shared by the catalog, requests and pricing.
func Operations() []Operation {
	var out []Operation
	add := func(id domain.MusicAction, title, group, kind string, sources, uploads int, max, format bool) {
		out = append(out, Operation{ID: id, Title: title, Group: group, OutputKind: kind, MinSources: sources, MaxSources: sources, MinUploads: uploads, MaxUploads: uploads, SupportsMax: max, SupportsCustomModel: max, SupportsAudioFormat: format})
	}
	add(domain.MusicActionGenerate, "Создать музыку", "create", "audio", 0, 0, true, true)
	add(domain.MusicActionLyrics, "Написать текст песни", "create", "text", 0, 0, false, false)
	add(domain.MusicActionUpsampleTags, "Улучшить описание стиля", "create", "text", 0, 0, false, false)
	add(domain.MusicActionSounds, "Создать звук", "create", "audio", 0, 0, false, true)
	add(domain.MusicActionInspo, "Вдохновиться записями", "create", "audio", 0, 1, true, true)
	out[len(out)-1].MaxUploads = 4
	add(domain.MusicActionUpload, "Загрузить запись", "create", "reference", 0, 1, false, false)
	add(domain.MusicActionUploadCover, "Кавер из записи", "create", "audio", 0, 1, true, true)
	add(domain.MusicActionUploadExtend, "Продолжить запись", "create", "audio", 0, 1, true, true)
	add(domain.MusicActionExtend, "Продолжить", "edit", "audio", 1, 0, true, true)
	add(domain.MusicActionCover, "Сделать кавер", "edit", "audio", 1, 0, true, true)
	add(domain.MusicActionRemaster, "Ремастеринг", "edit", "audio", 1, 0, false, true)
	add(domain.MusicActionReplaceSection, "Заменить фрагмент", "edit", "audio", 1, 0, true, true)
	add(domain.MusicActionRemoveSection, "Удалить фрагмент", "edit", "audio", 1, 0, false, false)
	add(domain.MusicActionCrop, "Обрезать", "edit", "audio", 1, 0, false, false)
	add(domain.MusicActionFadeIn, "Плавное начало", "edit", "audio", 1, 0, false, false)
	add(domain.MusicActionFadeOut, "Плавное затухание", "edit", "audio", 1, 0, false, false)
	add(domain.MusicActionAdjustSpeed, "Изменить скорость", "edit", "audio", 1, 0, false, false)
	add(domain.MusicActionConcat, "Склеить продолжение", "edit", "audio", 1, 0, false, true)
	add(domain.MusicActionMashup, "Смешать два трека", "edit", "audio", 2, 0, true, true)
	add(domain.MusicActionSample, "Взять сэмпл", "edit", "audio", 1, 0, true, true)
	add(domain.MusicActionStems, "Разделить вокал и музыку", "stems", "audio", 1, 0, false, true)
	add(domain.MusicActionStemsAll, "Разделить инструменты", "stems", "audio", 1, 0, false, true)
	add(domain.MusicActionAddVocals, "Добавить вокал", "stems", "audio", 1, 0, true, true)
	add(domain.MusicActionAddInstrumental, "Добавить музыку", "stems", "audio", 1, 0, true, true)
	add(domain.MusicActionAddStem, "Добавить инструмент", "stems", "audio", 1, 0, true, true)
	add(domain.MusicActionPersona, "Создать персону", "personalize", "persona", 1, 0, false, false)
	add(domain.MusicActionVoice, "Создать голос", "personalize", "voice", 0, 1, false, false)
	add(domain.MusicActionCreateModel, "Обучить модель", "personalize", "model", 0, 6, false, false)
	out[len(out)-1].MaxUploads = 24
	add(domain.MusicActionMIDI, "Получить MIDI", "tools", "document", 1, 0, false, false)
	add(domain.MusicActionAlignedLyrics, "Текст с таймкодами", "tools", "text", 1, 0, false, false)
	add(domain.MusicActionBPM, "Определить темп", "tools", "text", 1, 0, false, false)
	add(domain.MusicActionGenerateVideo, "Создать видеоклип", "tools", "video", 1, 0, false, false)
	add(domain.MusicActionExport, "Экспортировать", "tools", "audio", 1, 0, false, false)
	for i := range out {
		switch out[i].ID {
		case domain.MusicActionGenerate, domain.MusicActionUploadCover, domain.MusicActionUploadExtend, domain.MusicActionExtend, domain.MusicActionCover, domain.MusicActionMashup:
			out[i].SupportsPersona = true
		}
	}
	return out
}
func OperationByID(id domain.MusicAction) (Operation, bool) {
	for _, op := range Operations() {
		if op.ID == id {
			return op, true
		}
	}
	return Operation{}, false
}

// OperationsForModel prevents controls and source tasks from leaking between
// music APIs which happen to share the same reseller.
func OperationsForModel(model string) []Operation {
	if model == "lyria_3_5" {
		return []Operation{{ID: domain.MusicActionGenerate, Title: "Создать музыку", Group: "create", OutputKind: "audio"}}
	}
	if model == "suno_v6" || model == "suno_v6_wild" || model == "suno_v6_mini" {
		return Operations()
	}
	return nil
}

func SameSourceFamily(a, b string) bool {
	return a != "lyria_3_5" && b != "lyria_3_5" && len(OperationsForModel(a)) > 0 && len(OperationsForModel(b)) > 0
}
