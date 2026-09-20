package providermodels

import (
	"strings"

	"vk-ai-aggregator/internal/service/modelcontract"
)

const (
	mediaCandidateCheckedAt = "2026-09-16"
	apimartDocsBase         = "https://docs.apimart.ai/ru/api-reference/"
	apimartVideoEndpoint    = "POST /v1/videos/generations"
)

// MediaCandidateOperationFact is source-backed offline admission evidence for a
// pending media route. It is not a registry binding and does not enable traffic.
type MediaCandidateOperationFact struct {
	ID            string
	Kind          string
	Endpoint      string
	NativeVersion string
	SourceID      string
	InputModes    []string
	KnownLimits   []string
}

// MediaCandidateOperationFacts returns dated provider facts for pending media
// candidates. The facts intentionally keep unknown output attributes out of the
// active registry until live evidence exists.
func MediaCandidateOperationFacts(publicID string) []MediaCandidateOperationFact {
	candidate, ok := MediaCandidateByID(publicID)
	if !ok {
		return nil
	}
	switch publicID {
	case "happyhorse_1_0":
		return happyHorseOperationFacts(candidate.ModelCode, "happyhorse_1_0_generation", true)
	case "happyhorse_1_1":
		return happyHorseOperationFacts(candidate.ModelCode, "happyhorse_1_1_generation", false)
	case "skyreels_v4_fast", "skyreels_v4_std":
		return skyReelsOperationFacts(candidate.ModelCode)
	case "suno_v6", "suno_v6_wild", "suno_v6_mini":
		return sunoOperationFacts(sunoCandidateNativeVersion(candidate.ModelCode))
	default:
		return nil
	}
}

func buildMediaCandidateDraftContract(candidate MediaCandidate) modelcontract.Contract {
	facts := MediaCandidateOperationFacts(candidate.PublicID)
	return modelcontract.Contract{
		SchemaVersion:   1,
		PublicID:        candidate.PublicID,
		Provider:        string(candidate.Provider),
		ProviderModelID: candidate.ModelCode,
		Revision:        mediaCandidateCheckedAt,
		Endpoint:        mediaCandidateContractEndpoint(candidate, facts),
		Status:          "draft",
		Categories:      []string{"video-audio"},
		Sources:         mediaCandidateSources(candidate, facts),
		Operations:      mediaCandidateOperations(candidate, facts),
		Checks:          mediaCandidateDraftChecks(facts),
	}
}

func happyHorseOperationFacts(modelCode, sourceID string, includeEdit bool) []MediaCandidateOperationFact {
	facts := []MediaCandidateOperationFact{
		{
			ID:            "text_to_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      sourceID,
			InputModes:    []string{"prompt"},
			KnownLimits: []string{
				"model " + modelCode,
				"duration integer 3..15 seconds",
				"resolution 720P or 1080P",
				"size 16:9, 9:16, 1:1, 4:3, 3:4",
				"seed 0..2147483647 when present",
			},
		},
		{
			ID:            "image_to_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      sourceID,
			InputModes:    []string{"prompt optional", "first_frame_image"},
			KnownLimits: []string{
				"first_frame_image excludes image_urls and video_url",
				"image formats JPEG/JPG/PNG/BMP/WEBP",
				"image short side >=300px, aspect 1:2.5..2.5:1, <=10MB",
				"duration integer 3..15 seconds",
				"size ignored; output aspect inherited from first frame",
			},
		},
		{
			ID:            "reference_image_to_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      sourceID,
			InputModes:    []string{"prompt", "image_urls 1..9"},
			KnownLimits: []string{
				"image_urls excludes first_frame_image and standalone video_url",
				"image formats JPEG/JPG/PNG/BMP/WEBP",
				"reference image short side recommended >=720p, short/long >=0.4, <=10MB each",
				"duration integer 3..15 seconds",
				"resolution 720P or 1080P",
			},
		},
	}
	if includeEdit {
		facts = append(facts, MediaCandidateOperationFact{
			ID:            "video_edit",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      sourceID,
			InputModes:    []string{"prompt", "video_url", "image_urls 0..5 optional", "audio_setting auto|origin"},
			KnownLimits: []string{
				"video_url excludes first_frame_image and may combine only with image_urls",
				"video URL only, MP4/MOV recommended H.264, 3..60 seconds, cropped to first 15 seconds upstream",
				"output duration inherits source video; duration field ignored",
				"image_urls 0..5 style references in edit mode",
				"audio_setting only for edit; auto or origin",
			},
		})
	}
	return cloneOperationFacts(facts)
}

func skyReelsOperationFacts(modelCode string) []MediaCandidateOperationFact {
	facts := []MediaCandidateOperationFact{
		{
			ID:            "text_to_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt"},
			KnownLimits: []string{
				"model skyreels-v4-fast or skyreels-v4-std",
				"prompt <=1280 tokens",
				"duration 3..15 seconds",
				"resolution 480p, 720p, 1080p",
				"aspect_ratio 16:9, 4:3, 1:1, 9:16, 3:4",
			},
		},
		{
			ID:            "image_to_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt", "first_frame_image optional", "end_frame_image optional", "mid_frame_images 0..6"},
			KnownLimits: []string{
				"I2V fields cannot combine with Omni ref_images/ref_videos",
				"mid_frame_images tag must start with @ and appear in prompt",
				"mid_frame_images time_stamp is -1 or 0 < time_stamp < duration",
				"image URL formats documented as jpg/jpeg/png/gif/bmp",
				"aspect_ratio ignored; output aspect inherited from image input",
			},
		},
		{
			ID:            "omni_reference_images",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt", "ref_images type=image or grid"},
			KnownLimits: []string{
				"ref_images entries must share the same type",
				"type=image supports 1..3 groups with 1..5 image_urls each",
				"type=grid supports exactly one group with exactly one image_url",
				"tag must start with @ and appear in prompt",
				"Omni ref_images cannot combine with I2V fields",
			},
		},
		{
			ID:            "omni_reference_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt", "ref_videos type=reference", "ref_images type=image optional"},
			KnownLimits: []string{
				"ref_videos max count 1",
				"video_url MP4/MOV <=15 seconds",
				"type=reference output duration follows reference video, maximum 10 seconds",
				"type=reference may combine with ref_images.type=image",
				"aspect_ratio ignored when Omni uses ref_videos",
			},
		},
		{
			ID:            "omni_extend_video",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt", "ref_videos type=extend"},
			KnownLimits: []string{
				"ref_videos max count 1",
				"video_url MP4/MOV <=15 seconds",
				"type=extend cannot combine with ref_images",
				"type=extend billed by requested duration 3..15 seconds",
				"tag must start with @ and appear in prompt",
			},
		},
		{
			ID:            "omni_audio_sync",
			Kind:          "video",
			Endpoint:      apimartVideoEndpoint,
			NativeVersion: modelCode,
			SourceID:      "skyreels_v4_generation",
			InputModes:    []string{"prompt", "ref_images type=image", "audio_url"},
			KnownLimits: []string{
				"audio_url is supported only on ref_images.type=image",
				"audio_url duration <=15 seconds",
				"tag must start with @ and appear in prompt",
				"output audio/FPS are not live-verified and remain draft-only",
			},
		},
	}
	return cloneOperationFacts(facts)
}

func sunoOperationFacts(nativeVersion string) []MediaCandidateOperationFact {
	versioned := func(id, path, sourceID string, inputModes, limits []string) MediaCandidateOperationFact {
		return sunoFact(id, path, nativeVersion, sourceID, inputModes, limits)
	}
	unversioned := func(id, path, sourceID string, inputModes, limits []string) MediaCandidateOperationFact {
		return sunoFact(id, path, "", sourceID, inputModes, limits)
	}
	facts := []MediaCandidateOperationFact{
		versioned("generate", "/music/generations", "suno_generation", []string{"prompt or instrumental", "version or custom_model_id"}, []string{"version v6/v6-wild/v6-mini; custom_model_id excludes version and persona_id", "prompt <=3000 chars, lyrics <=5000 chars, style/tags <=1000 chars, title <=80 chars", "duration 10..360 seconds when present", "audio_format mp3/m4a/wav"}),
		unversioned("lyrics", "/music/generations/lyrics", "suno_lyrics", []string{"prompt"}, []string{"model suno", "prompt required", "lyrics_model optional"}),
		versioned("inspo", "/music/generations/inspo", "suno_inspo", []string{"audio_urls 1..4", "version or custom_model_id"}, []string{"audio_urls must be public audio URLs", "version v6/v6-wild/v6-mini by candidate", "audio_format mp3/m4a/wav"}),
		versioned("sounds", "/music/generations/sounds", "suno_sounds", []string{"prompt", "version"}, []string{"prompt required", "type/bpm/key optional", "audio_format mp3/m4a/wav", "custom_model_id not documented for sounds"}),
		unversioned("upsample_tags", "/music/generations/upsampleTags", "suno_upsample_tags", []string{"tags"}, []string{"tags required", "model suno"}),
		unversioned("upload", "/music/generations/uploadTask", "suno_upload", []string{"audioFilePath"}, []string{"public HTTP(S) audio URL only", "JSON endpoint does not upload binary files", "no version or audio_format"}),
		versioned("upload_cover", "/music/generations/uploadCover", "suno_upload_cover", []string{"audio_url", "version or custom_model_id"}, []string{"single public audio_url", "duration_s supported", "audio_format mp3/m4a/wav"}),
		versioned("upload_extend", "/music/generations/uploadExtend", "suno_upload_extend", []string{"audio_url", "continue_at", "version or custom_model_id"}, []string{"single public audio_url", "continue_at required", "duration_s supported", "audio_format mp3/m4a/wav"}),
		unversioned("create_model", "/music/generations/createModel", "suno_create_model", []string{"name", "audio_urls 6..24"}, []string{"6..24 direct public HTTP(S) audio URLs", "recommended MP3/WAV/M4A", "returns model_id for custom_model_id use"}),
		versioned("extend", "/music/generations/extend", "suno_extend", []string{"task_id", "audio_index optional", "continue_at", "version or custom_model_id"}, []string{"source task required", "continue_at required", "duration_s supported", "audio_format mp3/m4a/wav"}),
		versioned("cover", "/music/generations/coverSong", "suno_cover_song", []string{"task_id", "audio_index optional", "version or custom_model_id"}, []string{"source task required", "duration_s supported", "custom_model_id excludes version/persona_id", "audio_format mp3/m4a/wav"}),
		unversioned("remaster", "/music/generations/remaster", "suno_remaster", []string{"task_id", "audio_index optional"}, []string{"source task required", "variation_category optional", "audio_format mp3/m4a/wav"}),
		unversioned("stems", "/music/generations/stems", "suno_stems", []string{"task_id", "audio_index optional"}, []string{"source task required", "stem_type optional", "audio_format mp3/m4a/wav"}),
		unversioned("stems_all", "/music/generations/stemsAll", "suno_stems_all", []string{"task_id", "audio_index optional"}, []string{"source task required", "audio_format mp3/m4a/wav"}),
		versioned("add_vocals", "/music/generations/addVocals", "suno_add_vocals", []string{"task_id", "audio_index optional", "version or custom_model_id"}, []string{"source task required", "vocal_gender optional", "audio_format mp3/m4a/wav"}),
		versioned("add_instrumental", "/music/generations/addInstrumental", "suno_add_instrumental", []string{"task_id", "audio_index optional", "version or custom_model_id"}, []string{"source task required", "vocal_gender optional", "audio_format mp3/m4a/wav"}),
		versioned("add_stem", "/music/generations/addStem", "suno_add_stem", []string{"task_id", "audio_index optional", "version or custom_model_id"}, []string{"source task required", "audio_format mp3/m4a/wav"}),
		unversioned("voice", "/music/generations/createVoice", "suno_create_voice", []string{"audio_url"}, []string{"single public MP3/WAV audio_url", "returns voice_id"}),
		unversioned("persona", "/music/generations/persona", "suno_persona", []string{"task_id", "audio_index optional"}, []string{"source task required", "name/describe/styles optional", "vocal_start_s/vocal_end_s optional range"}),
		versioned("replace_section", "/music/generations/replaceMusic", "suno_replace_music", []string{"task_id", "audio_index optional", "start_s", "end_s", "version or custom_model_id"}, []string{"source task required", "start_s and end_s required", "end_s must be after start_s", "audio_format mp3/m4a/wav"}),
		unversioned("remove_section", "/music/generations/removeSection", "suno_remove_section", []string{"task_id", "audio_index optional", "start_s", "end_s"}, []string{"source task required", "start_s and end_s required", "end_s must be after start_s"}),
		unversioned("crop", "/music/generations/crop", "suno_crop", []string{"task_id", "audio_index optional", "start_s", "end_s"}, []string{"source task required", "start_s and end_s required", "end_s must be after start_s"}),
		unversioned("fade_in", "/music/generations/fadeIn", "suno_fade_in", []string{"task_id", "audio_index optional", "duration_s"}, []string{"source task required", "duration_s required"}),
		unversioned("fade_out", "/music/generations/fadeOut", "suno_fade_out", []string{"task_id", "audio_index optional", "duration_s"}, []string{"source task required", "duration_s required"}),
		unversioned("adjust_speed", "/music/generations/adjustSpeed", "suno_adjust_speed", []string{"task_id", "audio_index optional", "speed"}, []string{"source task required", "speed required and bounded 0.25..4", "keep_pitch optional"}),
		unversioned("concat", "/music/generations/concat", "suno_concat", []string{"task_id", "audio_index optional"}, []string{"source task required", "audio_format mp3/m4a/wav"}),
		versioned("mashup", "/music/generations/mashup", "suno_mashup", []string{"task_ids exactly 2", "audio_indexes optional", "version or custom_model_id"}, []string{"exactly two source tasks", "audio_indexes count must match task_ids when present", "audio_format mp3/m4a/wav"}),
		versioned("sample", "/music/generations/sample", "suno_sample", []string{"task_id", "audio_index optional", "start_s", "end_s", "version or custom_model_id"}, []string{"source must be uploadTask", "start_s and end_s required", "end_s must be after start_s", "audio_format mp3/m4a/wav"}),
		unversioned("midi", "/music/generations/midi", "suno_midi", []string{"task_id", "audio_index optional"}, []string{"source task required", "returns MIDI metadata"}),
		unversioned("aligned_lyrics", "/music/generations/alignedLyrics", "suno_aligned_lyrics", []string{"task_id", "audio_index optional"}, []string{"source task required", "returns aligned lyrics metadata"}),
		unversioned("bpm", "/music/generations/bpm", "suno_bpm", []string{"task_id", "audio_index optional"}, []string{"source task required", "returns BPM analysis"}),
		unversioned("generate_video", "/music/generations/generateMp4", "suno_generate_mp4", []string{"task_id", "audio_index optional"}, []string{"source task required", "creates music video artifact"}),
		unversioned("export", "/music/generations/download", "suno_wav", []string{"task_id", "audio_index optional", "formats or format"}, []string{"source task required", "format/formats required", "download page documents audio file export"}),
	}
	return cloneOperationFacts(facts)
}

func sunoFact(id, path, nativeVersion, sourceID string, inputModes, limits []string) MediaCandidateOperationFact {
	kind := "audio"
	switch id {
	case "lyrics", "upsample_tags", "upload", "create_model", "voice", "persona", "midi", "aligned_lyrics", "bpm":
		kind = "text"
	case "generate_video":
		kind = "video"
	}
	return MediaCandidateOperationFact{
		ID:            id,
		Kind:          kind,
		Endpoint:      "POST /v1" + path,
		NativeVersion: nativeVersion,
		SourceID:      sourceID,
		InputModes:    append([]string(nil), inputModes...),
		KnownLimits:   append([]string(nil), limits...),
	}
}

func sunoCandidateNativeVersion(modelCode string) string {
	switch strings.ToLower(strings.TrimSpace(modelCode)) {
	case "suno-v6-wild":
		return "v6-wild"
	case "suno-v6-mini":
		return "v6-mini"
	case "suno-v6":
		return "v6"
	default:
		return ""
	}
}

func mediaCandidateContractEndpoint(candidate MediaCandidate, facts []MediaCandidateOperationFact) string {
	if len(facts) == 0 {
		return ""
	}
	if candidate.Kind == "audio" {
		return "POST /v1/music/generations/*"
	}
	return facts[0].Endpoint
}

func mediaCandidateSources(candidate MediaCandidate, facts []MediaCandidateOperationFact) []modelcontract.Source {
	seen := map[string]bool{}
	var sources []modelcontract.Source
	add := func(id, path string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		sources = append(sources, modelcontract.Source{ID: id, URL: apimartDocsBase + path, CheckedAt: mediaCandidateCheckedAt})
	}
	if candidate.Kind == "audio" {
		add("suno_overview", "audios/suno/overview")
	}
	for _, fact := range facts {
		if path := mediaCandidateSourcePath(fact.SourceID); path != "" {
			add(fact.SourceID, path)
		}
	}
	if len(sources) == 0 && strings.TrimSpace(candidate.Documentation) != "" {
		sources = append(sources, modelcontract.Source{ID: "candidate_documentation", URL: candidate.Documentation, CheckedAt: candidate.CheckedAt})
	}
	return sources
}

func mediaCandidateSourcePath(sourceID string) string {
	switch sourceID {
	case "happyhorse_1_0_generation":
		return "videos/happyhorse-1.0/generation"
	case "happyhorse_1_1_generation":
		return "videos/happyhorse-1.1/generation"
	case "skyreels_v4_generation":
		return "videos/skyreels-v4/generation"
	case "suno_generation":
		return "audios/suno/generation"
	case "suno_lyrics":
		return "audios/suno/lyrics"
	case "suno_inspo":
		return "audios/suno/inspo"
	case "suno_sounds":
		return "audios/suno/sounds"
	case "suno_upsample_tags":
		return "audios/suno/upsample-tags"
	case "suno_upload":
		return "audios/suno/upload"
	case "suno_upload_cover":
		return "audios/suno/upload-cover"
	case "suno_upload_extend":
		return "audios/suno/upload-extend"
	case "suno_create_model":
		return "audios/suno/create-model"
	case "suno_extend":
		return "audios/suno/extend"
	case "suno_cover_song":
		return "audios/suno/cover-song"
	case "suno_remaster":
		return "audios/suno/remaster"
	case "suno_stems":
		return "audios/suno/stems"
	case "suno_stems_all":
		return "audios/suno/stems-all"
	case "suno_add_vocals":
		return "audios/suno/add-vocals"
	case "suno_add_instrumental":
		return "audios/suno/add-instrumental"
	case "suno_add_stem":
		return "audios/suno/add-stem"
	case "suno_create_voice":
		return "audios/suno/create-voice"
	case "suno_persona":
		return "audios/suno/persona"
	case "suno_replace_music":
		return "audios/suno/replace-music"
	case "suno_remove_section":
		return "audios/suno/remove-section"
	case "suno_crop":
		return "audios/suno/crop"
	case "suno_fade_in":
		return "audios/suno/fade-in"
	case "suno_fade_out":
		return "audios/suno/fade-out"
	case "suno_adjust_speed":
		return "audios/suno/adjust-speed"
	case "suno_concat":
		return "audios/suno/concat"
	case "suno_mashup":
		return "audios/suno/mashup"
	case "suno_sample":
		return "audios/suno/sample"
	case "suno_midi":
		return "audios/suno/midi"
	case "suno_aligned_lyrics":
		return "audios/suno/aligned-lyrics"
	case "suno_bpm":
		return "audios/suno/bpm"
	case "suno_generate_mp4":
		return "audios/suno/generate-mp4"
	case "suno_wav":
		return "audios/suno/wav"
	default:
		return ""
	}
}

func mediaCandidateOperations(candidate MediaCandidate, facts []MediaCandidateOperationFact) []modelcontract.Operation {
	ops := make([]modelcontract.Operation, 0, len(facts))
	for _, fact := range facts {
		switch candidate.Kind {
		case "video":
			ops = append(ops, mediaCandidateVideoOperation(candidate, fact))
		case "audio":
			ops = append(ops, mediaCandidateAudioOperation(fact))
		}
	}
	return ops
}

func mediaCandidateVideoOperation(candidate MediaCandidate, fact MediaCandidateOperationFact) modelcontract.Operation {
	inputs := explicitUnsupportedInputs()
	variants := mediaCandidateVideoVariants(candidate.ModelCode, fact.ID)
	startImage := "unsupported"
	endImage := "unsupported"
	switch candidate.PublicID {
	case "happyhorse_1_0", "happyhorse_1_1":
		switch fact.ID {
		case "image_to_video":
			inputs.Images = enabledImageInput(true, 1, []int{1}, happyHorseImageFormats(), 10<<20)
			inputs.MaxTotalBytes = 10 << 20
			startImage = "required"
		case "reference_image_to_video":
			inputs.Images = enabledImageInput(true, 9, integerRange(1, 9), happyHorseImageFormats(), 10<<20)
			inputs.MaxTotalBytes = 90 << 20
		case "video_edit":
			inputs.Video = enabledVideoInput(true, 1, []int{1}, videoMP4MOVFormats(), 100<<20, 60)
			inputs.Images = enabledImageInput(false, 5, integerRange(0, 5), happyHorseImageFormats(), 10<<20)
			inputs.MaxTotalBytes = 150 << 20
		}
	case "skyreels_v4_fast", "skyreels_v4_std":
		switch fact.ID {
		case "image_to_video":
			inputs.Images = enabledImageInput(true, 8, integerRange(1, 8), skyReelsImageFormats(), 0)
			startImage = "optional"
			endImage = "optional"
		case "omni_reference_images":
			inputs.Images = enabledImageInput(true, 15, integerRange(1, 15), skyReelsImageFormats(), 0)
		case "omni_reference_video":
			inputs.Video = enabledVideoInput(true, 1, []int{1}, videoMP4MOVFormats(), 0, 15)
			inputs.Images = enabledImageInput(false, 15, integerRange(0, 15), skyReelsImageFormats(), 0)
		case "omni_extend_video":
			inputs.Video = enabledVideoInput(true, 1, []int{1}, videoMP4MOVFormats(), 0, 15)
		case "omni_audio_sync":
			inputs.Images = enabledImageInput(true, 15, integerRange(1, 15), skyReelsImageFormats(), 0)
			inputs.Audio = modelcontract.Input{Support: modelcontract.Unknown}
		}
	}
	return modelcontract.Operation{
		ID:     fact.ID,
		Kind:   "video",
		Inputs: inputs,
		Video: &modelcontract.VideoOutput{
			Variants:       variants,
			DefaultVariant: 0,
			Formats:        []string{"video/mp4"},
			StartImage:     startImage,
			EndImage:       endImage,
		},
	}
}

func mediaCandidateAudioOperation(fact MediaCandidateOperationFact) modelcontract.Operation {
	inputs := explicitUnsupportedInputs()
	switch fact.ID {
	case "inspo":
		inputs.Audio = enabledAudioInput(true, 4, integerRange(1, 4), nil, 0, 0)
	case "upload", "upload_cover", "upload_extend":
		inputs.Audio = enabledAudioInput(true, 1, []int{1}, nil, 0, 0)
	case "voice":
		inputs.Audio = enabledAudioInput(true, 1, []int{1}, []modelcontract.FileFormat{{Extension: ".mp3", MIME: "audio/mpeg"}, {Extension: ".wav", MIME: "audio/wav"}}, 0, 0)
	case "create_model":
		inputs.Audio = enabledAudioInput(true, 24, integerRange(6, 24), nil, 0, 0)
	}
	op := modelcontract.Operation{ID: fact.ID, Kind: fact.Kind, Inputs: inputs}
	switch fact.Kind {
	case "text":
		op.Text = &modelcontract.TextOutput{}
	case "video":
		op.Video = &modelcontract.VideoOutput{Formats: []string{"video/mp4"}, StartImage: "unsupported", EndImage: "unsupported"}
	default:
		op.Audio = &modelcontract.AudioOutput{Tasks: []string{"music", "transform"}}
		// Requested 10..360 seconds is a generation target, not a proven bound
		// on every returned edit/export. Unknown output limits remain zero.
		if strings.Contains(strings.Join(fact.KnownLimits, " "), "audio_format mp3/m4a/wav") {
			op.Audio.Formats = []string{"audio/mpeg", "audio/mp4", "audio/wav"}
		}
	}
	return op
}

func mediaCandidateVideoVariants(modelCode, operationID string) []modelcontract.VideoVariant {
	var resolutions []string
	var aspects []string
	var durations []int
	switch strings.ToLower(strings.TrimSpace(modelCode)) {
	case "happyhorse-1.0", "happyhorse-1.1":
		resolutions = []string{"720P", "1080P"}
		aspects = []string{"16:9", "9:16", "1:1", "4:3", "3:4"}
		durations = integerRange(3, 15)
	case "skyreels-v4-fast", "skyreels-v4-std":
		resolutions = []string{"480p", "720p", "1080p"}
		aspects = []string{"16:9", "4:3", "1:1", "9:16", "3:4"}
		durations = integerRange(3, 15)
		if operationID == "omni_reference_video" {
			durations = integerRange(3, 10)
		}
	default:
		return nil
	}
	variants := make([]modelcontract.VideoVariant, 0, len(resolutions)*len(aspects)*len(durations))
	for _, resolution := range resolutions {
		for _, duration := range durations {
			for _, aspect := range aspects {
				variants = append(variants, modelcontract.VideoVariant{DurationSec: duration, Resolution: resolution, AspectRatio: aspect})
			}
		}
	}
	return variants
}

func mediaCandidateDraftChecks(facts []MediaCandidateOperationFact) []modelcontract.Check {
	checks := make([]modelcontract.Check, 0, len(facts))
	for _, fact := range facts {
		checks = append(checks, modelcontract.Check{Scenario: fact.ID + "/live-output", Status: "not_run", CheckedAt: mediaCandidateCheckedAt})
	}
	return checks
}

func explicitUnsupportedInputs() modelcontract.Inputs {
	unsupported := modelcontract.Input{Support: modelcontract.Unsupported}
	return modelcontract.Inputs{Images: unsupported, Video: unsupported, Audio: unsupported, Documents: unsupported}
}

func enabledImageInput(required bool, maxCount int, counts []int, formats []modelcontract.FileFormat, maxBytes int64) modelcontract.Input {
	return modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Required: required, Processing: "native", Formats: cloneFileFormats(formats), MaxCount: maxCount, AllowedCounts: append([]int(nil), counts...), MaxBytes: maxBytes}
}

func enabledVideoInput(required bool, maxCount int, counts []int, formats []modelcontract.FileFormat, maxBytes int64, maxDuration int) modelcontract.Input {
	return modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Required: required, Processing: "native", Formats: cloneFileFormats(formats), MaxCount: maxCount, AllowedCounts: append([]int(nil), counts...), MaxBytes: maxBytes, MaxDurationSec: maxDuration}
}

func enabledAudioInput(required bool, maxCount int, counts []int, formats []modelcontract.FileFormat, maxBytes int64, maxDuration int) modelcontract.Input {
	return modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Required: required, Processing: "native", Formats: cloneFileFormats(formats), MaxCount: maxCount, AllowedCounts: append([]int(nil), counts...), MaxBytes: maxBytes, MaxDurationSec: maxDuration}
}

func happyHorseImageFormats() []modelcontract.FileFormat {
	return []modelcontract.FileFormat{{Extension: ".jpg", MIME: "image/jpeg"}, {Extension: ".jpeg", MIME: "image/jpeg"}, {Extension: ".png", MIME: "image/png"}, {Extension: ".bmp", MIME: "image/bmp"}, {Extension: ".webp", MIME: "image/webp"}}
}

func skyReelsImageFormats() []modelcontract.FileFormat {
	return []modelcontract.FileFormat{{Extension: ".jpg", MIME: "image/jpeg"}, {Extension: ".jpeg", MIME: "image/jpeg"}, {Extension: ".png", MIME: "image/png"}, {Extension: ".gif", MIME: "image/gif"}, {Extension: ".bmp", MIME: "image/bmp"}}
}

func videoMP4MOVFormats() []modelcontract.FileFormat {
	return []modelcontract.FileFormat{{Extension: ".mp4", MIME: "video/mp4"}, {Extension: ".mov", MIME: "video/quicktime"}}
}

func cloneFileFormats(in []modelcontract.FileFormat) []modelcontract.FileFormat {
	out := make([]modelcontract.FileFormat, len(in))
	copy(out, in)
	return out
}

func cloneOperationFacts(in []MediaCandidateOperationFact) []MediaCandidateOperationFact {
	out := make([]MediaCandidateOperationFact, len(in))
	for i, fact := range in {
		out[i] = fact
		out[i].InputModes = append([]string(nil), fact.InputModes...)
		out[i].KnownLimits = append([]string(nil), fact.KnownLimits...)
	}
	return out
}
