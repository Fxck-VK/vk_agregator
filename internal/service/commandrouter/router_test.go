package commandrouter

import (
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestRouterParse(t *testing.T) {
	r := New()

	tests := []struct {
		name      string
		input     string
		wantType  domain.CommandType
		wantOp    domain.OperationType
		wantMod   domain.Modality
		wantPromp string
		wantArg   string
	}{
		{
			name:      "image command with prompt",
			input:     "/image neon city at night",
			wantType:  domain.CommandImageGenerate,
			wantOp:    domain.OperationImageGenerate,
			wantMod:   domain.ModalityImage,
			wantPromp: "neon city at night",
		},
		{
			name:      "video command case insensitive",
			input:     "/VIDEO girl walking in Tokyo",
			wantType:  domain.CommandVideoGenerate,
			wantOp:    domain.OperationVideoGenerate,
			wantMod:   domain.ModalityVideo,
			wantPromp: "girl walking in Tokyo",
		},
		{
			name:      "edit command",
			input:     "/edit make the sky purple",
			wantType:  domain.CommandImageEdit,
			wantOp:    domain.OperationImageEdit,
			wantMod:   domain.ModalityImage,
			wantPromp: "make the sky purple",
		},
		{
			name:     "balance control command",
			input:    "/balance",
			wantType: domain.CommandBalance,
		},
		{
			name:     "status with job id arg",
			input:    "/status 1c2d",
			wantType: domain.CommandStatus,
			wantArg:  "1c2d",
		},
		{
			name:     "cancel with job id arg",
			input:    "/cancel 99",
			wantType: domain.CommandCancel,
			wantArg:  "99",
		},
		{
			name:     "help command",
			input:    "  /help  ",
			wantType: domain.CommandHelp,
		},
		{
			name:      "plain text becomes text generate",
			input:     "напиши пост для VK",
			wantType:  domain.CommandTextAsk,
			wantOp:    domain.OperationTextGenerate,
			wantMod:   domain.ModalityText,
			wantPromp: "напиши пост для VK",
		},
		{
			name:      "unknown slash command falls back to text generate",
			input:     "/foo bar",
			wantType:  domain.CommandTextAsk,
			wantOp:    domain.OperationTextGenerate,
			wantMod:   domain.ModalityText,
			wantPromp: "/foo bar",
		},
		{
			name:      "image command without prompt",
			input:     "/image",
			wantType:  domain.CommandImageGenerate,
			wantOp:    domain.OperationImageGenerate,
			wantMod:   domain.ModalityImage,
			wantPromp: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Parse(tt.input)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Operation != tt.wantOp {
				t.Errorf("Operation = %q, want %q", got.Operation, tt.wantOp)
			}
			if got.Modality != tt.wantMod {
				t.Errorf("Modality = %q, want %q", got.Modality, tt.wantMod)
			}
			if got.Prompt != tt.wantPromp {
				t.Errorf("Prompt = %q, want %q", got.Prompt, tt.wantPromp)
			}
			if got.Arg != tt.wantArg {
				t.Errorf("Arg = %q, want %q", got.Arg, tt.wantArg)
			}
		})
	}
}

func TestResultCreatesJob(t *testing.T) {
	r := New()

	jobCommands := []string{"/image cat", "/video dog", "/edit fix", "just text"}
	for _, in := range jobCommands {
		if !r.Parse(in).CreatesJob() {
			t.Errorf("expected %q to create a job", in)
		}
	}

	controlCommands := []string{"/balance", "/status 1", "/cancel 1", "/help"}
	for _, in := range controlCommands {
		if r.Parse(in).CreatesJob() {
			t.Errorf("expected %q to NOT create a job", in)
		}
	}
}
