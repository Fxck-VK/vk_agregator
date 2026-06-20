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
	// CommandMenuImageText selects text-to-image mode in the VK image menu.
	CommandMenuImageText CommandType = "menu.image.text"
	// CommandMenuImageNanoBanana2 selects Nano Banana 2 image generation.
	CommandMenuImageNanoBanana2 CommandType = "menu.image.nano_banana_2"
	// CommandMenuImageGPTImage2 selects GPT Image 2 image generation.
	CommandMenuImageGPTImage2 CommandType = "menu.image.gpt_image_2"
	// CommandMenuImageReference selects image generation with a reference photo.
	CommandMenuImageReference CommandType = "menu.image.reference"
	// CommandMenuVideo explains how to generate a video.
	CommandMenuVideo CommandType = "menu.video"
	// CommandMenuVideoPrunaAI selects the currently wired PrunaAI video model.
	CommandMenuVideoPrunaAI CommandType = "menu.video.prunaai"
	// CommandMenuVideoSora2 selects Sora 2 in the VK video model picker.
	CommandMenuVideoSora2 CommandType = "menu.video.sora_2"
	// CommandMenuVideoSora2Start opens the Sora 2 generation placeholder.
	CommandMenuVideoSora2Start CommandType = "menu.video.sora_2.start"
	// CommandMenuVideoSora2Examples opens Sora 2 prompt examples.
	CommandMenuVideoSora2Examples CommandType = "menu.video.sora_2.examples"
	// CommandMenuVideoKling21 selects Kling v2.1 in the VK video model picker.
	CommandMenuVideoKling21 CommandType = "menu.video.kling_v2_1"
	// CommandMenuVideoKling21Start opens the Kling v2.1 generation placeholder.
	CommandMenuVideoKling21Start CommandType = "menu.video.kling_v2_1.start"
	// CommandMenuVideoKling21Examples opens Kling v2.1 prompt examples.
	CommandMenuVideoKling21Examples CommandType = "menu.video.kling_v2_1.examples"
	// CommandMenuVideoSeedance1 selects Seedance 1 in the VK video model picker.
	CommandMenuVideoSeedance1 CommandType = "menu.video.seedance_1"
	// CommandMenuVideoSeedance1Lite selects the Seedance 1 Lite variant.
	CommandMenuVideoSeedance1Lite CommandType = "menu.video.seedance_1.lite"
	// CommandMenuVideoSeedance1Pro selects the Seedance 1 Pro variant.
	CommandMenuVideoSeedance1Pro CommandType = "menu.video.seedance_1.pro"
	// CommandMenuVideoHailuo02 selects Hailuo v0.2 in the VK video model picker.
	CommandMenuVideoHailuo02 CommandType = "menu.video.haiuo_v0_2"
	// CommandMenuVideoHailuo02Standard selects the regular Hailuo v0.2 variant.
	CommandMenuVideoHailuo02Standard CommandType = "menu.video.haiuo_v0_2.standard"
	// CommandMenuVideoHailuo02Fast selects the fast Hailuo v0.2 variant.
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
		CommandMenuImageText,
		CommandMenuImageNanoBanana2,
		CommandMenuImageGPTImage2,
		CommandMenuImageReference,
		CommandMenuVideo,
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
