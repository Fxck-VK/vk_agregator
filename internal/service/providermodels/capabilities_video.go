package providermodels

import (
	"sort"

	"vk-ai-aggregator/internal/domain"
)

const (
	videoDurationSelected       = "selected"
	videoDurationAutomatic      = "automatic"
	videoDurationReferenceVideo = "reference_video"

	videoAudioOptional       = "optional"
	videoAudioGenerated      = "generated"
	videoAudioSilent         = "silent"
	videoAudioPreserveSource = "preserve_source"
	videoAudioUnknown        = "unknown"

	frameRequired    = "required"
	frameOptional    = "optional"
	frameUnsupported = "unsupported"
	frameUnknown     = "unknown"
)

// Evidence checked 2026-09-14: docs/VIDEO_GENERATION.md,
// docs/ARCHITECTURE.md, internal/service/videorouter/catalog.go,
// internal/service/providermodels/{registry,turbo_h3,omni_video,kling_veo,media_contracts}.go,
// internal/adapter/provider/{apimart,poyo,runway}/*.go, and
// internal/worker/{worker,motion_video}.go. Comments stay source-only because
// returned capabilities must not expose provider ids, costs, URLs, or secrets.
func videoCapabilities(m VideoRoute) *ModelCapabilities {
	return &ModelCapabilities{
		SchemaVersion: 1,
		API: CapabilityProfile{
			Video: videoCapability(m, false),
			Notes: videoCapabilityNotes(m, false),
		},
		Application: CapabilityProfile{
			Video: videoCapability(m, true),
			Notes: videoCapabilityNotes(m, true),
		},
	}
}

// VideoCapabilitiesForOptions returns capabilities for a runtime/pricing-filtered
// public projection. Only the application-facing request options are narrowed;
// the native API profile remains the immutable adapter contract. Automatic routes
// keep their application placeholder duration as a placeholder, not as a selected
// provider duration.
func VideoCapabilitiesForOptions(publicID string, durations []int, resolutions []string) *ModelCapabilities {
	caps := Capabilities(publicID)
	if caps == nil || caps.Application.Video == nil {
		return caps
	}
	filterApplicationVideoOptions(caps.Application.Video, durations, resolutions)
	return caps
}

func videoCapability(m VideoRoute, application bool) *VideoCapabilities {
	spec := m.Spec
	alias := spec.Alias
	images := videoImageInputCapability(alias, spec, application)
	videos := videoInputCapability(alias, application)
	imageMax := 0
	if images.MaxCount != nil {
		imageMax = *images.MaxCount
	}
	counts := videoAllowedImageCounts(alias, spec, imageMax, application)
	if images.Support == Supported && images.MaxCount == nil {
		counts = nil
	}
	return &VideoCapabilities{
		Images:             images,
		Videos:             videos,
		AllowedImageCounts: counts,
		Duration:           videoDurationCapability(alias, spec, application),
		Resolutions:        videoResolutionOptions(alias, spec),
		QualityModes:       videoQualityModes(alias, spec),
		AspectRatios:       append([]string(nil), spec.AllowedAspectRatios...),
		Audio:              videoAudioCapability(alias),
		StartFrame:         videoStartFrame(alias, spec, application),
		EndFrame:           videoEndFrame(alias, spec, application),
	}
}

func videoImageInputCapability(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec, application bool) InputCapability {
	if !spec.SupportsReferenceImage || spec.MaxReferenceImages <= 0 {
		return noInput()
	}
	maximum := spec.MaxReferenceImages
	extensions := []string{"jpg", "jpeg", "png"}
	if !application {
		extensions = videoAPIImageExtensions(alias)
		if alias == domain.VideoRouteSeedance25 || alias == domain.VideoRouteKlingO3Standard {
			// Product limits and the adapter's broad safety guard differ; neither
			// proves the native reference limit for this endpoint.
			return inputCapability(Supported, nil, extensions...)
		}
	}
	return inputCapability(Supported, integer(maximum), extensions...)
}

func videoInputCapability(alias domain.VideoRouteAlias, application bool) InputCapability {
	if alias == domain.VideoRouteKling26Motion {
		return inputCapability(Supported, integer(1), "mp4", "mov")
	}
	if application {
		return noInput()
	}
	return unknownInput()
}

func videoAPIImageExtensions(alias domain.VideoRouteAlias) []string {
	switch alias {
	case domain.VideoRouteKling30Turbo:
		return []string{"jpg", "jpeg", "png"}
	case domain.VideoRouteMiniMaxH3,
		domain.VideoRouteKlingV3,
		domain.VideoRouteKling26Motion,
		domain.VideoRouteVeo31Fast,
		domain.VideoRouteVeo31Quality,
		domain.VideoRouteOmni11Flash,
		domain.VideoRouteOmni11FlashExt,
		domain.VideoRouteSeedance25:
		return []string{"jpg", "jpeg", "png", "webp"}
	case domain.VideoRouteHailuo23Fast, domain.VideoRouteHailuo23Standard:
		return []string{"jpg", "jpeg", "png", "gif", "webp"}
	case domain.VideoRouteKlingO3Standard,
		domain.VideoRouteSeedance20Fast,
		domain.VideoRouteRunwayGen4Turbo,
		domain.VideoRouteRunwayGen45:
		return nil
	default:
		return nil
	}
}

func videoAllowedImageCounts(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec, max int, application bool) []int {
	if max <= 0 {
		return []int{0}
	}
	if len(spec.AllowedReferenceImageCounts) > 0 {
		return sortedUniqueInts(spec.AllowedReferenceImageCounts)
	}
	if spec.RequiresStartImage || alias == domain.VideoRouteKling26Motion {
		return []int{1}
	}
	return intSequence(0, max)
}

func videoDurationCapability(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec, application bool) DurationCapability {
	switch alias {
	case domain.VideoRouteOmni11Flash:
		return DurationCapability{Mode: videoDurationAutomatic, MinSeconds: integer(3), MaxSeconds: integer(10)}
	case domain.VideoRouteKling26Motion:
		return DurationCapability{
			Mode:           videoDurationReferenceVideo,
			MinSeconds:     integer(3),
			MaxSeconds:     integer(30),
			AllowedSeconds: intSequence(3, 30),
			ByOrientation:  map[string]int{"image": 10, "video": 30},
		}
	}
	return selectedVideoDuration(spec.AllowedDurationsSec, spec.ResolutionDurationsSec)
}

func selectedVideoDuration(values []int, byResolution map[string][]int) DurationCapability {
	allowed := sortedUniqueInts(values)
	out := DurationCapability{Mode: videoDurationSelected, AllowedSeconds: allowed, ByResolution: copyDurationMap(byResolution)}
	setDurationBounds(&out)
	return out
}

func videoResolutionOptions(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec) []string {
	if alias == domain.VideoRouteKling26Motion {
		// The 4K media safety ceiling is not a selectable or promised output
		// resolution. The documented request exposes only std/pro modes.
		return nil
	}
	return append([]string(nil), spec.AllowedResolutions...)
}

func videoQualityModes(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec) []string {
	if alias != domain.VideoRouteKling26Motion {
		return nil
	}
	return append([]string(nil), spec.AllowedResolutions...)
}

func videoAudioCapability(alias domain.VideoRouteAlias) VideoAudioCapability {
	switch alias {
	case domain.VideoRouteKlingV3:
		return VideoAudioCapability{Mode: videoAudioOptional, Selectable: true}
	case domain.VideoRouteKling26Motion:
		return VideoAudioCapability{Mode: videoAudioPreserveSource, Selectable: true}
	case domain.VideoRouteOmni11Flash, domain.VideoRouteOmni11FlashExt, domain.VideoRouteSeedance25:
		return VideoAudioCapability{Mode: videoAudioGenerated, Selectable: false}
	case domain.VideoRouteKlingO3Standard, domain.VideoRouteSeedance20Fast:
		return VideoAudioCapability{Mode: videoAudioSilent, Selectable: false}
	default:
		return VideoAudioCapability{Mode: videoAudioUnknown, Selectable: false}
	}
}

func videoStartFrame(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec, application bool) string {
	if spec.RequiresStartImage {
		return frameRequired
	}
	switch alias {
	case domain.VideoRouteKling30Turbo, domain.VideoRouteMiniMaxH3, domain.VideoRouteHailuo23Standard, domain.VideoRouteKlingV3, domain.VideoRouteVeo31Fast, domain.VideoRouteVeo31Quality, domain.VideoRouteOmni11FlashExt:
		return frameOptional
	}
	if !application && spec.SupportsReferenceImage && spec.MaxReferenceImages > 0 {
		return frameUnknown
	}
	return frameUnsupported
}

func videoEndFrame(alias domain.VideoRouteAlias, spec domain.VideoRouteSpec, application bool) string {
	switch alias {
	case domain.VideoRouteKlingV3, domain.VideoRouteVeo31Fast, domain.VideoRouteVeo31Quality:
		return frameOptional
	}
	if application {
		return frameUnsupported
	}
	if spec.SupportsReferenceImage {
		return frameUnknown
	}
	return frameUnsupported
}

func videoCapabilityNotes(m VideoRoute, application bool) []string {
	alias := m.Spec.Alias
	notes := []string{
		"Профиль показывает проверенные ограничения выбора; стоимость считается отдельно перед запуском.",
	}
	if application {
		notes = append(notes, "В интерфейсе изображения загружаются как JPEG/PNG; видео MP4/MOV доступно только для Motion Control.")
	} else {
		notes = append(notes, "Указан проверенный набор параметров API. Неподтверждённые форматы и ограничения отмечены отдельно; отсутствие значения не означает отсутствие поддержки.")
	}
	switch alias {
	case domain.VideoRouteOmni11Flash:
		notes = append(notes, "Фактическую длительность выбирает модель; 10 секунд используется только для расчета запроса.")
	case domain.VideoRouteOmni11FlashExt:
		notes = append(notes, "Два изображения не принимаются: можно без изображения, с одним кадром или с тремя референсами.")
	case domain.VideoRouteSeedance25:
		notes = append(notes, "Звук генерируется автоматически; переключатель звука не показывается.")
	case domain.VideoRouteKlingV3:
		notes = append(notes, "Можно добавить до двух кадров; стартовый и конечный кадр зависят от числа изображений.")
	case domain.VideoRouteKling26Motion:
		notes = append(notes, "Требуется одно изображение и одно видео; при ориентации image длительность ограничена 10 секундами, при video — 30.")
		notes = append(notes, "Выбирается режим Std или Pro. Точное разрешение результата в пикселях не подтверждено.")
	case domain.VideoRouteVeo31Fast, domain.VideoRouteVeo31Quality:
		notes = append(notes, "Одно или два изображения используются как кадры; у Fast три изображения считаются референсами.")
	case domain.VideoRouteRunwayGen4Turbo:
		notes = append(notes, "Доступен выбор 5 или 10 секунд.")
	case domain.VideoRouteHailuo23Fast, domain.VideoRouteHailuo23Standard:
		notes = append(notes, "Маршрут сейчас скрыт от продажи; для 1080p допустимо только 6 секунд.")
	case domain.VideoRouteKlingO3Standard:
		notes = append(notes, "Звук выключен. В приложении можно добавить одно референсное изображение; полный лимит API пока не подтверждён.")
	}
	return notes
}

func filterApplicationVideoOptions(video *VideoCapabilities, durations []int, resolutions []string) {
	if len(durations) > 0 && video.Duration.Mode != videoDurationAutomatic {
		video.Duration.AllowedSeconds = filterInts(video.Duration.AllowedSeconds, sortedUniqueInts(durations))
		setDurationBounds(&video.Duration)
	}
	if len(resolutions) > 0 {
		allowedResolutions := uniqueStrings(resolutions)
		if len(video.QualityModes) > 0 {
			video.QualityModes = filterStrings(video.QualityModes, allowedResolutions)
		} else {
			video.Resolutions = filterStrings(video.Resolutions, allowedResolutions)
		}
		if len(video.Duration.ByResolution) > 0 {
			video.Duration.ByResolution = filterDurationMap(video.Duration.ByResolution, allowedResolutions, video.Duration.AllowedSeconds)
			if byResolutionAllowed := unionDurationMap(video.Duration.ByResolution); len(byResolutionAllowed) > 0 {
				video.Duration.AllowedSeconds = byResolutionAllowed
				if video.Duration.Mode != videoDurationAutomatic {
					setDurationBounds(&video.Duration)
				}
			}
		}
	}
}

func setDurationBounds(duration *DurationCapability) {
	if len(duration.AllowedSeconds) == 0 {
		duration.MinSeconds = nil
		duration.MaxSeconds = nil
		return
	}
	duration.MinSeconds = integer(duration.AllowedSeconds[0])
	duration.MaxSeconds = integer(duration.AllowedSeconds[len(duration.AllowedSeconds)-1])
}

func sortedUniqueInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func intSequence(first, last int) []int {
	if last < first {
		return nil
	}
	out := make([]int, 0, last-first+1)
	for value := first; value <= last; value++ {
		out = append(out, value)
	}
	return out
}

func copyDurationMap(in map[string][]int) map[string][]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]int, len(in))
	for key, values := range in {
		out[key] = sortedUniqueInts(values)
	}
	return out
}

func filterDurationMap(in map[string][]int, allowedKeys []string, allowedDurations []int) map[string][]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]int, len(in))
	for _, key := range allowedKeys {
		values, ok := in[key]
		if !ok {
			continue
		}
		filtered := append([]int(nil), values...)
		if len(allowedDurations) > 0 {
			filtered = filterInts(filtered, allowedDurations)
		}
		if len(filtered) > 0 {
			out[key] = filtered
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func unionDurationMap(in map[string][]int) []int {
	if len(in) == 0 {
		return nil
	}
	values := make([]int, 0)
	for _, durations := range in {
		values = append(values, durations...)
	}
	return sortedUniqueInts(values)
}

func filterInts(values []int, allowed []int) []int {
	if len(values) == 0 || len(allowed) == 0 {
		return nil
	}
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := allowedSet[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func filterStrings(values []string, allowed []string) []string {
	if len(values) == 0 || len(allowed) == 0 {
		return nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowedSet[value]; ok {
			out = append(out, value)
		}
	}
	return out
}
