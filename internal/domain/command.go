package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CommandType is the normalized intent parsed from a VK message by the Command
// Router. It is provider-agnostic and never triggers an AI call by itself.
type CommandType string

const (
	// CommandTextAsk asks a text model a question.
	CommandTextAsk CommandType = "text.ask"
	// CommandImageGenerate generates an image from a prompt.
	CommandImageGenerate CommandType = "image.generate"
	// CommandImageEdit edits an attached image.
	CommandImageEdit CommandType = "image.edit"
	// CommandVideoGenerate generates a video from a prompt.
	CommandVideoGenerate CommandType = "video.generate"
	// CommandVideoImageToVideo animates an attached image.
	CommandVideoImageToVideo CommandType = "video.image_to_video"
	// CommandVideoExtend extends an existing video.
	CommandVideoExtend CommandType = "video.extend"
	// CommandAudioTTS synthesizes speech from text.
	CommandAudioTTS CommandType = "audio.tts"
	// CommandStart opens the VK product menu.
	CommandStart CommandType = "start"
	// CommandShowMenu reopens the VK product menu after onboarding.
	CommandShowMenu CommandType = "show_menu"
	// CommandBalance reports the user's credit balance.
	CommandBalance CommandType = "balance"
	// CommandAccount reports the user's account state.
	CommandAccount CommandType = "account"
	// CommandTopUp starts the balance top-up flow.
	CommandTopUp CommandType = "top_up"
	// CommandMenuText explains how to ask the text model.
	CommandMenuText CommandType = "menu.text"
	// CommandMenuImage explains how to generate an image.
	CommandMenuImage CommandType = "menu.image"
	// CommandMenuImageSelect selects an enabled public image model alias from
	// the backend-owned product catalog.
	CommandMenuImageSelect CommandType = "menu.image.select"
	// CommandMenuImageText is a legacy VK image-model command kept for stale
	// payload/text compatibility. New buttons use CommandMenuImageSelect.
	CommandMenuImageText CommandType = "menu.image.text"
	// CommandMenuImageNanoBanana2 is a legacy VK image-model command kept for
	// stale payload/text compatibility. New buttons use CommandMenuImageSelect.
	CommandMenuImageNanoBanana2 CommandType = "menu.image.nano_banana_2"
	// CommandMenuImageDeepInfraSeedream is a legacy VK image-model command kept
	// for stale payload/text compatibility. New buttons use CommandMenuImageSelect.
	CommandMenuImageDeepInfraSeedream CommandType = "menu.image.deepinfra_seedream_4_5"
	// CommandMenuImageDeepInfraSDXL is a legacy VK image-model command kept for
	// stale payload/text compatibility. New buttons use CommandMenuImageSelect.
	CommandMenuImageDeepInfraSDXL CommandType = "menu.image.deepinfra_sdxl_turbo"
	// CommandMenuImageGPTImage2 is a legacy VK image-model command kept for stale
	// payload/text compatibility. New buttons use CommandMenuImageSelect.
	CommandMenuImageGPTImage2 CommandType = "menu.image.gpt_image_2"
	// CommandMenuImageQuality1K is a legacy VK quality command. New buttons use
	// CommandMenuImageQualitySelect.
	CommandMenuImageQuality1K CommandType = "menu.image.quality.1k"
	// CommandMenuImageQuality2K is a legacy VK quality command. New buttons use
	// CommandMenuImageQualitySelect.
	CommandMenuImageQuality2K CommandType = "menu.image.quality.2k"
	// CommandMenuImageQuality4K is a legacy VK quality command. New buttons use
	// CommandMenuImageQualitySelect.
	CommandMenuImageQuality4K CommandType = "menu.image.quality.4k"
	// CommandMenuImageQualitySelect selects a public image quality alias from
	// the backend-owned product catalog.
	CommandMenuImageQualitySelect CommandType = "menu.image.quality.select"
	// CommandMenuImageBackToQuality reopens the quality picker for the active image model.
	CommandMenuImageBackToQuality CommandType = "menu.image.back_to_quality"
	// CommandMenuImageReference selects image generation with a reference photo.
	CommandMenuImageReference CommandType = "menu.image.reference"
	// CommandMenuVideo explains how to generate a video.
	CommandMenuVideo CommandType = "menu.video"
	// CommandMenuVideoRouteSelect selects an enabled public video route alias
	// from the backend-owned product catalog.
	CommandMenuVideoRouteSelect CommandType = "menu.video.route.select"
	// CommandMenuVideoDurationSelect selects an allowed public duration for the
	// active video route.
	CommandMenuVideoDurationSelect CommandType = "menu.video.duration.select"
	// CommandMenuVideoPrunaAI is a disabled legacy VK video command.
	CommandMenuVideoPrunaAI CommandType = "menu.video.prunaai"
	// CommandMenuVideoSora2 is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoSora2 CommandType = "menu.video.sora_2"
	// CommandMenuVideoSora2Start is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoSora2Start CommandType = "menu.video.sora_2.start"
	// CommandMenuVideoSora2Examples is a legacy VK examples command kept for
	// stale payload/text compatibility.
	CommandMenuVideoSora2Examples CommandType = "menu.video.sora_2.examples"
	// CommandMenuVideoKling21 is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoKling21 CommandType = "menu.video.kling_v2_1"
	// CommandMenuVideoKling21Start is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoKling21Start CommandType = "menu.video.kling_v2_1.start"
	// CommandMenuVideoKling21Examples is a legacy VK examples command kept for
	// stale payload/text compatibility.
	CommandMenuVideoKling21Examples CommandType = "menu.video.kling_v2_1.examples"
	// CommandMenuVideoSeedance1 is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoSeedance1 CommandType = "menu.video.seedance_1"
	// CommandMenuVideoSeedance1Lite is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoSeedance1Lite CommandType = "menu.video.seedance_1.lite"
	// CommandMenuVideoSeedance1Pro is a disabled legacy VK video command.
	CommandMenuVideoSeedance1Pro CommandType = "menu.video.seedance_1.pro"
	// CommandMenuVideoHailuo02 is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoHailuo02 CommandType = "menu.video.haiuo_v0_2"
	// CommandMenuVideoHailuo02Standard is a legacy VK video command kept for
	// stale payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoHailuo02Standard CommandType = "menu.video.haiuo_v0_2.standard"
	// CommandMenuVideoHailuo02Fast is a legacy VK video command kept for stale
	// payload/text compatibility. New buttons use CommandMenuVideoRouteSelect.
	CommandMenuVideoHailuo02Fast CommandType = "menu.video.haiuo_v0_2.fast"
	// CommandMenuStudents opens the student/schoolchild help section.
	CommandMenuStudents CommandType = "menu.students"
	// CommandMenuStudentSolver opens the task solver student flow.
	CommandMenuStudentSolver CommandType = "menu.students.solver"
	// CommandMenuStudentPresentation opens the presentation generation placeholder.
	CommandMenuStudentPresentation CommandType = "menu.students.presentation"
	// CommandMenuStudentReport opens the report generation placeholder.
	CommandMenuStudentReport CommandType = "menu.students.report"
	// CommandMenuStudentQA opens the student question-answer flow.
	CommandMenuStudentQA CommandType = "menu.students.qa"
	// CommandCancel cancels an in-flight job.
	CommandCancel CommandType = "cancel"
	// CommandStatus reports the status of a job.
	CommandStatus CommandType = "status"
	// CommandHelp shows usage help.
	CommandHelp CommandType = "help"
	// CommandUnknown is an unrecognized command requiring clarification.
	CommandUnknown CommandType = "unknown"
)

// Valid reports whether the command type is one of the known commands.
func (c CommandType) Valid() bool {
	switch c {
	case CommandTextAsk,
		CommandImageGenerate,
		CommandImageEdit,
		CommandVideoGenerate,
		CommandVideoImageToVideo,
		CommandVideoExtend,
		CommandAudioTTS,
		CommandStart,
		CommandShowMenu,
		CommandBalance,
		CommandAccount,
		CommandTopUp,
		CommandMenuText,
		CommandMenuImage,
		CommandMenuImageSelect,
		CommandMenuImageText,
		CommandMenuImageNanoBanana2,
		CommandMenuImageDeepInfraSeedream,
		CommandMenuImageDeepInfraSDXL,
		CommandMenuImageGPTImage2,
		CommandMenuImageQuality1K,
		CommandMenuImageQuality2K,
		CommandMenuImageQuality4K,
		CommandMenuImageQualitySelect,
		CommandMenuImageBackToQuality,
		CommandMenuImageReference,
		CommandMenuVideo,
		CommandMenuVideoRouteSelect,
		CommandMenuVideoDurationSelect,
		CommandMenuVideoPrunaAI,
		CommandMenuVideoSora2,
		CommandMenuVideoSora2Start,
		CommandMenuVideoSora2Examples,
		CommandMenuVideoKling21,
		CommandMenuVideoKling21Start,
		CommandMenuVideoKling21Examples,
		CommandMenuVideoSeedance1,
		CommandMenuVideoSeedance1Lite,
		CommandMenuVideoSeedance1Pro,
		CommandMenuVideoHailuo02,
		CommandMenuVideoHailuo02Standard,
		CommandMenuVideoHailuo02Fast,
		CommandMenuStudents,
		CommandMenuStudentSolver,
		CommandMenuStudentPresentation,
		CommandMenuStudentReport,
		CommandMenuStudentQA,
		CommandCancel,
		CommandStatus,
		CommandHelp,
		CommandUnknown:
		return true
	default:
		return false
	}
}

// CreatesJob reports whether the command produces a billable AI Job (as opposed
// to control commands like balance, status or help).
func (c CommandType) CreatesJob() bool {
	switch c {
	case CommandTextAsk,
		CommandImageGenerate,
		CommandImageEdit,
		CommandVideoGenerate,
		CommandVideoImageToVideo,
		CommandVideoExtend,
		CommandAudioTTS:
		return true
	default:
		return false
	}
}

// Command is the normalized representation of a single user request. It is the
// bridge between a raw VK inbound event and a Job.
type Command struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// UserID is the user who issued the command.
	UserID uuid.UUID `json:"user_id"`
	// VKPeerID is the VK conversation the command came from.
	VKPeerID int64 `json:"vk_peer_id"`
	// InboundEventID links back to the raw VK event for idempotency.
	InboundEventID uuid.UUID `json:"inbound_event_id"`
	// Type is the parsed command intent.
	Type CommandType `json:"type"`
	// RawText is the original user message text.
	RawText string `json:"raw_text"`
	// Args holds parsed arguments and options (mode, quality, model hint...).
	Args json.RawMessage `json:"args"`
	// AttachmentArtifactIDs are artifacts extracted from VK attachments.
	AttachmentArtifactIDs []uuid.UUID `json:"attachment_artifact_ids"`
	// IdempotencyKey deduplicates command creation for the same message.
	IdempotencyKey string `json:"idempotency_key"`
	// CorrelationID links the command to its inbound event and resulting job.
	CorrelationID string `json:"correlation_id"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
