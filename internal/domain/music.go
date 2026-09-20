package domain

import "encoding/json"

// MusicAction is the provider-neutral operation requested from a music adapter.
// It intentionally uses product-owned action IDs, not provider endpoint names.
type MusicAction string

const (
	MusicActionGenerate        MusicAction = "generate"
	MusicActionLyrics          MusicAction = "lyrics"
	MusicActionInspo           MusicAction = "inspo"
	MusicActionSounds          MusicAction = "sounds"
	MusicActionUpsampleTags    MusicAction = "upsample_tags"
	MusicActionUpload          MusicAction = "upload"
	MusicActionUploadCover     MusicAction = "upload_cover"
	MusicActionUploadExtend    MusicAction = "upload_extend"
	MusicActionCreateModel     MusicAction = "create_model"
	MusicActionExtend          MusicAction = "extend"
	MusicActionCover           MusicAction = "cover"
	MusicActionRemaster        MusicAction = "remaster"
	MusicActionStems           MusicAction = "stems"
	MusicActionStemsAll        MusicAction = "stems_all"
	MusicActionAddVocals       MusicAction = "add_vocals"
	MusicActionAddInstrumental MusicAction = "add_instrumental"
	MusicActionAddStem         MusicAction = "add_stem"
	MusicActionVoice           MusicAction = "voice"
	MusicActionPersona         MusicAction = "persona"
	MusicActionReplaceSection  MusicAction = "replace_section"
	MusicActionRemoveSection   MusicAction = "remove_section"
	MusicActionCrop            MusicAction = "crop"
	MusicActionFadeIn          MusicAction = "fade_in"
	MusicActionFadeOut         MusicAction = "fade_out"
	MusicActionAdjustSpeed     MusicAction = "adjust_speed"
	MusicActionConcat          MusicAction = "concat"
	MusicActionMashup          MusicAction = "mashup"
	MusicActionSample          MusicAction = "sample"
	MusicActionMIDI            MusicAction = "midi"
	MusicActionAlignedLyrics   MusicAction = "aligned_lyrics"
	MusicActionBPM             MusicAction = "bpm"
	MusicActionGenerateVideo   MusicAction = "generate_video"
	MusicActionExport          MusicAction = "export"
)

// Valid reports whether the action is part of the current non-deprecated music
// contract. Deprecated provider operations such as Suno vox stay invalid.
func (a MusicAction) Valid() bool {
	switch a {
	case MusicActionGenerate,
		MusicActionLyrics,
		MusicActionInspo,
		MusicActionSounds,
		MusicActionUpsampleTags,
		MusicActionUpload,
		MusicActionUploadCover,
		MusicActionUploadExtend,
		MusicActionCreateModel,
		MusicActionExtend,
		MusicActionCover,
		MusicActionRemaster,
		MusicActionStems,
		MusicActionStemsAll,
		MusicActionAddVocals,
		MusicActionAddInstrumental,
		MusicActionAddStem,
		MusicActionVoice,
		MusicActionPersona,
		MusicActionReplaceSection,
		MusicActionRemoveSection,
		MusicActionCrop,
		MusicActionFadeIn,
		MusicActionFadeOut,
		MusicActionAdjustSpeed,
		MusicActionConcat,
		MusicActionMashup,
		MusicActionSample,
		MusicActionMIDI,
		MusicActionAlignedLyrics,
		MusicActionBPM,
		MusicActionGenerateVideo,
		MusicActionExport:
		return true
	default:
		return false
	}
}

// MusicRequest is the provider-neutral request shape for music creation,
// editing, transforms and analysis. Ephemeral provider-reference URLs are kept
// in memory only and must not be serialized into durable snapshots.
type MusicRequest struct {
	Action MusicAction `json:"action"`

	Custom         *bool    `json:"custom,omitempty"`
	Instrumental   *bool    `json:"instrumental,omitempty"`
	Prompt         string   `json:"prompt,omitempty"`
	Lyrics         string   `json:"lyrics,omitempty"`
	GPTDescription string   `json:"gpt_description,omitempty"`
	Title          string   `json:"title,omitempty"`
	Style          string   `json:"style,omitempty"`
	Tags           string   `json:"tags,omitempty"`
	NegativeTags   string   `json:"negative_tags,omitempty"`
	AutoLyrics     *bool    `json:"auto_lyrics,omitempty"`
	PersonaID      string   `json:"persona_id,omitempty"`
	VocalGender    string   `json:"vocal_gender,omitempty"`
	StyleWeight    *float64 `json:"style_weight,omitempty"`
	Weirdness      *float64 `json:"weirdness,omitempty"`
	AudioWeight    *float64 `json:"audio_weight,omitempty"`
	Variety        string   `json:"variety,omitempty"`
	MaxMode        *bool    `json:"max_mode,omitempty"`
	AudioFormat    string   `json:"audio_format,omitempty"`
	DurationSec    int      `json:"duration_sec,omitempty"`

	CustomModelID string `json:"custom_model_id,omitempty"`
	LyricsModel   string `json:"lyrics_model,omitempty"`

	SourceTaskID       string   `json:"source_task_id,omitempty"`
	SourceAudioIndex   int      `json:"source_audio_index,omitempty"`
	SourceTaskIDs      []string `json:"source_task_ids,omitempty"`
	SourceAudioIndexes []int    `json:"source_audio_indexes,omitempty"`

	ContinueAtSec *float64 `json:"continue_at_sec,omitempty"`
	StartSec      *float64 `json:"start_sec,omitempty"`
	EndSec        *float64 `json:"end_sec,omitempty"`
	Speed         *float64 `json:"speed,omitempty"`
	KeepPitch     *bool    `json:"keep_pitch,omitempty"`

	Name              string   `json:"name,omitempty"`
	Description       string   `json:"description,omitempty"`
	Styles            string   `json:"styles,omitempty"`
	VocalStartSec     *float64 `json:"vocal_start_sec,omitempty"`
	VocalEndSec       *float64 `json:"vocal_end_sec,omitempty"`
	StemType          string   `json:"stem_type,omitempty"`
	VariationCategory string   `json:"variation_category,omitempty"`
	InfillLyrics      string   `json:"infill_lyrics,omitempty"`
	SoundType         string   `json:"sound_type,omitempty"`
	BPM               int      `json:"bpm,omitempty"`
	Key               string   `json:"key,omitempty"`
	Formats           []string `json:"formats,omitempty"`
	Format            string   `json:"format,omitempty"`

	AudioURL  string   `json:"-"`
	AudioURLs []string `json:"-"`
}

// MusicResult is normalized provider output metadata. Artifact URLs stay
// transient and are carried separately by ProviderTaskResult.OutputURLs.
type MusicResult struct {
	Tracks        []MusicTrackResult    `json:"tracks,omitempty"`
	Lyrics        []MusicLyricsResult   `json:"lyrics,omitempty"`
	Artifacts     []MusicArtifactResult `json:"artifacts,omitempty"`
	MIDI          *MusicMIDIResult      `json:"midi,omitempty"`
	Persona       *MusicPersonaResult   `json:"persona,omitempty"`
	Model         *MusicModelResult     `json:"model,omitempty"`
	Voice         *MusicVoiceResult     `json:"voice,omitempty"`
	BPM           *MusicBPMResult       `json:"bpm,omitempty"`
	UpsampledTags string                `json:"upsampled_tags,omitempty"`
	Alignment     json.RawMessage       `json:"alignment,omitempty"`
	Waveform      json.RawMessage       `json:"waveform,omitempty"`
}

type MusicTrackResult struct {
	OriginalAudioIndex int      `json:"original_audio_index,omitempty"`
	AudioID            string   `json:"audio_id,omitempty"`
	Status             string   `json:"status,omitempty"`
	Title              string   `json:"title,omitempty"`
	Lyrics             string   `json:"lyrics,omitempty"`
	Tags               string   `json:"tags,omitempty"`
	DisplayTags        string   `json:"display_tags,omitempty"`
	NegativeTags       string   `json:"negative_tags,omitempty"`
	StyleWeight        *float64 `json:"style_weight,omitempty"`
	Weirdness          *float64 `json:"weirdness,omitempty"`
	AudioWeight        *float64 `json:"audio_weight,omitempty"`
	DurationSec        float64  `json:"duration_sec,omitempty"`
	AudioURL           string   `json:"-"`
	ImageURL           string   `json:"-"`
	ImageLargeURL      string   `json:"-"`
	VideoURL           string   `json:"-"`
}

type MusicLyricsResult struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
	Tags  string `json:"tags,omitempty"`
}

type MusicArtifactResult struct {
	Kind               string `json:"kind,omitempty"`
	Format             string `json:"format,omitempty"`
	OriginalAudioIndex int    `json:"original_audio_index,omitempty"`
	URL                string `json:"-"`
}

type MusicMIDIResult struct {
	State       string          `json:"state,omitempty"`
	Instruments json.RawMessage `json:"instruments,omitempty"`
}

type MusicPersonaResult struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Styles      string `json:"styles,omitempty"`
}

type MusicModelResult struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type MusicVoiceResult struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type MusicBPMResult struct {
	Average float64 `json:"average,omitempty"`
	Minimum float64 `json:"minimum,omitempty"`
	Maximum float64 `json:"maximum,omitempty"`
}
