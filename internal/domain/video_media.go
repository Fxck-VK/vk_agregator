package domain

// VideoMediaMode is the normalized media route selected by trusted worker code.
type VideoMediaMode string

const (
	VideoMediaModeText           VideoMediaMode = "text"
	VideoMediaModeImage          VideoMediaMode = "image"
	VideoMediaModeReferenceImage VideoMediaMode = "reference_image"
	VideoMediaModeEdit           VideoMediaMode = "edit"
	VideoMediaModeOmni           VideoMediaMode = "omni"
)

// VideoReferenceImageType identifies how reference image groups are interpreted.
type VideoReferenceImageType string

const (
	VideoReferenceImageTypeImage VideoReferenceImageType = "image"
	VideoReferenceImageTypeGrid  VideoReferenceImageType = "grid"
)

// VideoReferenceVideoType identifies how an input video affects generation.
type VideoReferenceVideoType string

const (
	VideoReferenceVideoTypeEdit      VideoReferenceVideoType = "edit"
	VideoReferenceVideoTypeReference VideoReferenceVideoType = "reference"
	VideoReferenceVideoTypeExtend    VideoReferenceVideoType = "extend"
)

// VideoAudioSetting is limited to documented provider-native audio controls.
type VideoAudioSetting string

const (
	VideoAudioSettingAuto   VideoAudioSetting = "auto"
	VideoAudioSettingOrigin VideoAudioSetting = "origin"
)

// VideoMediaRequest carries worker-owned video media inputs. Fetchable URLs are
// ephemeral because they can be signed provider-reference URLs; they must not be
// serialized into durable snapshots.
type VideoMediaRequest struct {
	Mode                 VideoMediaMode             `json:"mode,omitempty"`
	StartFrame           *VideoFrame                `json:"start_frame,omitempty"`
	EndFrame             *VideoFrame                `json:"end_frame,omitempty"`
	KeyFrames            []VideoKeyFrame            `json:"key_frames,omitempty"`
	ReferenceImageGroups []VideoReferenceImageGroup `json:"reference_image_groups,omitempty"`
	ReferenceVideos      []VideoReferenceVideo      `json:"reference_videos,omitempty"`
	Audio                *VideoMediaAudio           `json:"audio,omitempty"`
	Seed                 *int                       `json:"seed,omitempty"`
	PromptOptimizer      *bool                      `json:"prompt_optimizer,omitempty"`
}

type VideoFrame struct {
	URL string `json:"-"`
}

type VideoKeyFrame struct {
	Tag          string `json:"tag,omitempty"`
	URL          string `json:"-"`
	TimeStampSec *int   `json:"time_stamp_sec,omitempty"`
}

type VideoReferenceImageGroup struct {
	Tag              string                  `json:"tag,omitempty"`
	Type             VideoReferenceImageType `json:"type,omitempty"`
	URLs             []string                `json:"-"`
	AudioURL         string                  `json:"-"`
	AudioDurationSec int                     `json:"audio_duration_sec,omitempty"`
}

type VideoReferenceVideo struct {
	Tag         string                  `json:"tag,omitempty"`
	Type        VideoReferenceVideoType `json:"type,omitempty"`
	URL         string                  `json:"-"`
	DurationSec int                     `json:"duration_sec,omitempty"`
}

type VideoMediaAudio struct {
	Setting VideoAudioSetting `json:"setting,omitempty"`
	URL     string            `json:"-"`
}
