package providermodels

import (
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcontract"
)

// MediaCandidate is researched metadata, not a runtime routing permission.
// Candidates are kept outside Bindings/legacy and can enter StaticRegistry only
// together with a verified, fingerprint-bound admission contract.
type MediaCandidate struct {
	PublicID      string
	Name          string
	Kind          string
	Provider      domain.ProviderName
	ModelCode     string
	Documentation string
	CheckedAt     string
	Capabilities  ModelCapabilities
}

func MediaCandidates() []MediaCandidate {
	out := []MediaCandidate{}
	for _, v := range []struct {
		id, name, native string
		sky, edit        bool
	}{
		{"happyhorse_1_0", "HappyHorse 1.0", "happyhorse-1.0", false, true},
		{"happyhorse_1_1", "HappyHorse 1.1", "happyhorse-1.1", false, false},
		{"skyreels_v4_fast", "SkyReels V4 Fast", "skyreels-v4-fast", true, false},
		{"skyreels_v4_std", "SkyReels V4 Std", "skyreels-v4-std", true, false},
	} {
		api := VideoCapabilities{Images: inputCapability(Supported, integer(9), "jpg", "jpeg", "png", "bmp", "webp"), Videos: noInput(), Duration: DurationCapability{Mode: "selected", MinSeconds: integer(3), MaxSeconds: integer(15)}, Resolutions: []string{"720p", "1080p"}, AspectRatios: []string{"16:9", "9:16", "1:1", "4:3", "3:4"}, Audio: VideoAudioCapability{Mode: "unknown"}, StartFrame: "optional", EndFrame: "unsupported"}
		notes := []string{"Возможности API подтверждены документацией. Запуск ожидает проверки реального результата.", "FPS и наличие звука в результате ещё не подтверждены."}
		page := "videos/" + v.native + "/generation"
		if v.edit {
			api.Videos = inputCapability(Supported, integer(1), "mp4", "mov")
			notes = append(notes, "Режим редактирования наследует длительность входа, не более 15 секунд; до 5 референсов.")
		}
		if v.sky {
			api.Images = inputCapability(Supported, integer(15))
			api.Videos = inputCapability(Supported, integer(1), "mp4", "mov")
			api.Resolutions = []string{"480p", "720p", "1080p"}
			api.EndFrame = "optional"
			notes = append(notes, "Режим кадров: начальный, конечный и до 6 промежуточных. Режим Omni: до 3 групп по 5 изображений; сетка — один файл.", "Видеореференс: вход до 15 секунд, результат до 10 секунд; продолжение задаётся отдельно. Режимы кадров и Omni несовместимы.", "Документация противоречива относительно звука; наличие звука остаётся неизвестным.")
			page = "videos/skyreels-v4/generation"
		}
		app := VideoCapabilities{Images: noInput(), Videos: noInput(), Duration: DurationCapability{Mode: "unknown"}, Audio: VideoAudioCapability{Mode: "unknown"}, StartFrame: "unsupported", EndFrame: "unsupported"}
		out = append(out, MediaCandidate{PublicID: v.id, Name: v.name, Kind: "video", Provider: domain.ProviderAPIMart, ModelCode: v.native, Documentation: "https://docs.apimart.ai/ru/api-reference/" + page, CheckedAt: "2026-09-16", Capabilities: ModelCapabilities{SchemaVersion: 1, API: CapabilityProfile{Video: &api, Notes: notes}, Application: CapabilityProfile{Video: &app, Notes: []string{"Ожидает допуска; генерация и загрузка вложений выключены."}}}})
	}
	for _, v := range []struct{ id, name, native string }{{"suno_v6", "Suno V6", "suno-v6"}, {"suno_v6_wild", "Suno V6 Wild", "suno-v6-wild"}, {"suno_v6_mini", "Suno V6 Mini", "suno-v6-mini"}} {
		out = append(out, MediaCandidate{PublicID: v.id, Name: v.name, Kind: "audio", Provider: domain.ProviderAPIMart, ModelCode: v.native, Documentation: "https://docs.apimart.ai/ru/api-reference/audios/suno/overview", CheckedAt: "2026-09-16", Capabilities: ModelCapabilities{SchemaVersion: 1, API: CapabilityProfile{Audio: &AudioCapabilities{Audio: inputCapability(Supported, nil), Videos: noInput(), Output: "music"}, Notes: []string{"Лимиты входа зависят от операции: вдохновение — 1–4 записи, обучение модели — 6–24, загрузка для кавера/продолжения — менее 8 минут.", "Количество выходных треков определяется фактическим ответом. Форматы и режим Max доступны не во всех операциях."}}, Application: CapabilityProfile{Audio: &AudioCapabilities{Audio: noInput(), Videos: noInput(), Output: "music"}, Notes: []string{"Ожидает допуска; платные операции и загрузка выключены."}}}})
	}
	return out
}

func MediaCandidateByID(id string) (MediaCandidate, bool) {
	for _, c := range MediaCandidates() {
		if c.PublicID == id {
			return c, true
		}
	}
	return MediaCandidate{}, false
}

func IsPendingMediaRoute(provider domain.ProviderName, model string) bool {
	for _, c := range MediaCandidates() {
		if c.Provider == provider && c.ModelCode == model {
			return true
		}
	}
	return false
}

// MediaCandidateAdmitted does not accept a feature flag as verification.
func (r Registry) MediaCandidateAdmitted(id, operation string) bool {
	c, ok := r.Contracts[id]
	if !ok || c.ValidateReady() != nil || r.ValidateOnboarding() != nil {
		return false
	}
	for _, b := range r.Bindings() {
		if b.PublicID != id {
			continue
		}
		for _, op := range c.Operations {
			if op.ID == operation {
				return true
			}
		}
	}
	return false
}

// MusicReferencesAdmitted enables the private reference gateway only after a
// music operation has canonical admission. Credentials are still mandatory.
func MusicReferencesAdmitted() bool {
	r := StaticRegistry()
	for _, candidate := range MediaCandidates() {
		if candidate.Kind != "audio" {
			continue
		}
		for _, operation := range r.Contracts[candidate.PublicID].Operations {
			if r.MediaCandidateAdmitted(candidate.PublicID, operation.ID) {
				return true
			}
		}
	}
	return false
}

// DraftMediaContract supplies dated source facts without fabricating live checks,
// output FPS, active registry fingerprints or rollout readiness.
func DraftMediaContract(c MediaCandidate) modelcontract.Contract {
	return buildMediaCandidateDraftContract(c)
}
