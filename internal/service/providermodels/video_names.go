package providermodels

import "vk-ai-aggregator/internal/domain"

// VideoName is the display name shared by the catalog and capability export.
func VideoName(alias domain.VideoRouteAlias) string {
	switch alias {
	case domain.VideoRouteKling30Turbo:
		return "Kling 3.0 Turbo"
	case domain.VideoRouteMiniMaxH3:
		return "MiniMax H3"
	case domain.VideoRouteKlingV3:
		return "Kling V3"
	case domain.VideoRouteKling26Motion:
		return "Kling 2.6 Motion Control"
	case domain.VideoRouteVeo31Fast:
		return "Veo 3.1 Fast"
	case domain.VideoRouteVeo31Quality:
		return "Veo 3.1 Quality"
	case domain.VideoRouteVeo31Lite:
		return "Veo 3.1 Lite"
	case domain.VideoRouteOmni11Flash:
		return "Gemini Omni 1.1 Flash"
	case domain.VideoRouteOmni11FlashExt:
		return "Gemini Omni 1.1 Flash EXT"
	case domain.VideoRouteSeedance25:
		return "Seedance 2.5"
	case domain.VideoRouteHailuo23Fast:
		return "Hailuo 2.3 Fast"
	case domain.VideoRouteHailuo23Standard:
		return "Hailuo 2.3 Standard"
	case domain.VideoRouteKlingO3Standard:
		return "Kling O3 Standard"
	case domain.VideoRouteRunwayGen4Turbo:
		return "Runway Gen-4 Turbo"
	case domain.VideoRouteSeedance20Fast:
		return "Seedance 2.0 Fast"
	case domain.VideoRouteRunwayGen45:
		return "Runway Gen-4.5"
	case domain.VideoRouteMockTextToVideo:
		return "Mock Video Loadtest"
	default:
		return "Видео"
	}
}
