package providermodels

import "vk-ai-aggregator/internal/domain"

const nextMediaCandidateCheckedAt = "2026-09-20"

func nextMediaCandidates() []MediaCandidate {
	return []MediaCandidate{
		nextVideoCandidate("wan_3_0", "Wan 3.0 Video", "wan3.0-video", "https://docs.apimart.ai/en/api-reference/videos/wan3.0-video/generation", 2, 30, []string{"480p", "720p", "1080p"}, 10, 5),
		nextVideoCandidate("vidu_q3_pro", "Vidu Q3 Pro", "viduq3-pro", "https://docs.apimart.ai/en/api-reference/videos/vidu-q3-pro/generation", 1, 16, []string{"540p", "720p", "1080p"}, 2, 0),
		nextImageCandidate(),
		nextAudioCandidate("lyria_3_5", "Lyria 3.5", "flowmusic-lyria-3.5", "https://docs.apimart.ai/en/api-reference/audios/flow-music/music-lyria-3-5", "music"),
		nextAudioCandidate("gpt_4o_mini_tts", "GPT-4o Mini TTS", "gpt-4o-mini-tts", "https://docs.apimart.ai/en/api-reference/audios/tts", "speech"),
		nextAudioCandidate("whisper_1", "Whisper 1", "whisper-1", "https://docs.apimart.ai/en/api-reference/audios/whisper-1", "transcribe"),
	}
}

func nextVideoCandidate(id, name, native, doc string, minDuration, maxDuration int, resolutions []string, maxImages, maxVideos int) MediaCandidate {
	imageInput := inputCapability(Supported, integer(maxImages), "jpg", "jpeg", "png", "bmp", "webp")
	videoInput := noInput()
	if maxVideos > 0 {
		videoInput = inputCapability(Supported, integer(maxVideos), "mp4", "mov")
	}
	api := VideoCapabilities{
		Images:       imageInput,
		Videos:       videoInput,
		Duration:     DurationCapability{Mode: videoDurationSelected, MinSeconds: integer(minDuration), MaxSeconds: integer(maxDuration), AllowedSeconds: intSequence(minDuration, maxDuration)},
		Resolutions:  append([]string(nil), resolutions...),
		AspectRatios: []string{"16:9", "4:3", "1:1", "3:4", "9:16"},
		Audio:        VideoAudioCapability{Mode: videoAudioOptional, Selectable: true},
		StartFrame:   frameOptional,
		EndFrame:     frameOptional,
	}
	if maxImages > 0 {
		api.AllowedImageCounts = intSequence(0, maxImages)
	}
	app := VideoCapabilities{
		Images:      noInput(),
		Videos:      noInput(),
		Duration:    DurationCapability{Mode: videoDurationSelected, MinSeconds: integer(minDuration), MaxSeconds: integer(maxDuration), AllowedSeconds: intSequence(minDuration, maxDuration)},
		Resolutions: append([]string(nil), resolutions...),
		AspectRatios: []string{
			"16:9", "4:3", "1:1", "3:4", "9:16",
		},
		Audio:      VideoAudioCapability{Mode: videoAudioUnknown},
		StartFrame: frameUnsupported,
		EndFrame:   frameUnsupported,
	}
	return MediaCandidate{
		PublicID:      id,
		Name:          name,
		Kind:          "video",
		Provider:      domain.ProviderAPIMart,
		ModelCode:     native,
		Documentation: doc,
		CheckedAt:     nextMediaCandidateCheckedAt,
		Capabilities: ModelCapabilities{SchemaVersion: 1,
			API:         CapabilityProfile{Video: &api, Notes: []string{"Возможности API взяты из документации APIMart. FPS и фактическая аудиодорожка результата не проверены live."}},
			Application: CapabilityProfile{Video: &app, Notes: []string{"Ожидает допуска; публичная заявка будет text-to-video без кадров и вложений."}},
		},
	}
}

func nextImageCandidate() MediaCandidate {
	image := ImageCapabilities{Images: noInput(), AspectRatios: nextImagenAspectRatios(), MaxOutputCount: integer(1)}
	return MediaCandidate{
		PublicID:      "imagen_4_0",
		Name:          "Imagen 4.0",
		Kind:          "image",
		Provider:      domain.ProviderAPIMart,
		ModelCode:     "imagen-4.0-apimart",
		Documentation: "https://docs.apimart.ai/en/api-reference/images/imagen-4.0-apimart/generation",
		CheckedAt:     nextMediaCandidateCheckedAt,
		Capabilities: ModelCapabilities{SchemaVersion: 1,
			API:         CapabilityProfile{Image: &image, Notes: []string{"Text-to-image only, one image per request. Pixel dimensions are not documented and remain absent."}},
			Application: CapabilityProfile{Image: &image, Notes: []string{"Ожидает допуска; референсы и редактирование выключены."}},
		},
	}
}

func nextAudioCandidate(id, name, native, doc, output string) MediaCandidate {
	apiAudio := noInput()
	if id == "whisper_1" {
		apiAudio = inputCapability(Supported, integer(1), "mp3", "mp4", "mpeg", "mpga", "m4a", "wav", "webm")
	}
	audio := AudioCapabilities{Audio: apiAudio, Videos: noInput(), Output: output}
	app := AudioCapabilities{Audio: noInput(), Videos: noInput(), Output: output}
	return MediaCandidate{
		PublicID:      id,
		Name:          name,
		Kind:          "audio",
		Provider:      domain.ProviderAPIMart,
		ModelCode:     native,
		Documentation: doc,
		CheckedAt:     nextMediaCandidateCheckedAt,
		Capabilities: ModelCapabilities{SchemaVersion: 1,
			API:         CapabilityProfile{Audio: &audio, Notes: []string{"Документированные возможности APIMart сохранены как pending facts; retail billing is not active."}},
			Application: CapabilityProfile{Audio: &app, Notes: []string{"Ожидает допуска; операция выключена, цена для speech/transcribe не опубликована."}},
		},
	}
}

func nextImagenAspectRatios() []string {
	return []string{"1:1", "4:3", "3:4", "16:9", "9:16"}
}
