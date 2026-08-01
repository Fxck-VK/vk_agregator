package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestJobValidateResultContract(t *testing.T) {
	accountID := uuid.New()
	validTarget := &DeliveryTarget{Channel: ChannelVKBot, RecipientRef: "peer:2000000001"}

	tests := []struct {
		name string
		job  Job
		want bool
	}{
		{
			name: "account history requires canonical account and no target",
			job: Job{
				AccountID:  accountID,
				ResultMode: ResultModeAccountHistory,
			},
			want: true,
		},
		{
			name: "account history rejects missing account",
			job: Job{
				ResultMode: ResultModeAccountHistory,
			},
			want: false,
		},
		{
			name: "account history rejects delivery target",
			job: Job{
				AccountID:      accountID,
				ResultMode:     ResultModeAccountHistory,
				DeliveryTarget: validTarget,
			},
			want: false,
		},
		{
			name: "external push requires valid target",
			job: Job{
				ResultMode: ResultModeExternalPush,
			},
			want: false,
		},
		{
			name: "external push accepts valid target without account provenance",
			job: Job{
				ResultMode:     ResultModeExternalPush,
				DeliveryTarget: validTarget,
			},
			want: true,
		},
		{
			name: "legacy unknown remains readable without target",
			job: Job{
				ResultMode: ResultModeLegacyUnknown,
			},
			want: true,
		},
		{
			name: "legacy unknown rejects a new delivery target",
			job: Job{
				ResultMode:     ResultModeLegacyUnknown,
				DeliveryTarget: validTarget,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.ValidateResultContract() == nil; got != tt.want {
				t.Fatalf("ValidateResultContract success = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChannelContextValidationIsBoundedAndOpaque(t *testing.T) {
	if !(ChannelContext{Channel: ChannelWeb, RecipientRef: "account-session", ThreadRef: "conversation-1"}).Valid() {
		t.Fatal("valid bounded context rejected")
	}
	if (ChannelContext{Channel: Channel("email"), RecipientRef: "opaque"}).Valid() {
		t.Fatal("unsupported channel accepted")
	}
	if (DeliveryTarget{Channel: ChannelVKBot}).Valid() {
		t.Fatal("target without opaque recipient reference accepted")
	}
	if (DeliveryTarget{Channel: ChannelVKBot, RecipientRef: string(make([]byte, MaxChannelReferenceLength+1))}).Valid() {
		t.Fatal("oversized opaque target reference accepted")
	}
	for _, channel := range []Channel{ChannelVKMiniApp, ChannelWeb} {
		if (DeliveryTarget{Channel: channel, RecipientRef: "opaque-recipient"}).Valid() {
			t.Fatalf("non-publishable channel %q accepted as delivery target", channel)
		}
	}
}

func TestDeliveryValidateTargetPreservesLegacyNilTarget(t *testing.T) {
	if err := (Delivery{}).ValidateTarget(); err != nil {
		t.Fatalf("legacy delivery without target rejected: %v", err)
	}
	if err := (Delivery{Target: &DeliveryTarget{Channel: ChannelWeb, RecipientRef: "account"}}).ValidateTarget(); err == nil {
		t.Fatal("web delivery target accepted")
	}
}

func TestResultReadyCanTransitionDirectlyToSucceeded(t *testing.T) {
	if !JobStatusResultReady.CanTransitionTo(JobStatusSucceeded) {
		t.Fatal("account-history finalization must allow result_ready -> succeeded")
	}
}
