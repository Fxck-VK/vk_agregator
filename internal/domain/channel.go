package domain

import (
	"errors"
	"fmt"
	"strings"
)

// MaxChannelReferenceLength bounds opaque transport references before they are
// persisted. They are adapter-owned provenance, never authorization input.
const MaxChannelReferenceLength = 512

// Channel identifies a trusted product transport. The set is deliberately
// closed so an unknown value cannot silently route work to a publisher.
type Channel string

const (
	// ChannelVKBot represents the VK Bot transport.
	ChannelVKBot Channel = "vk_bot"
	// ChannelVKMiniApp represents the signed VK Mini App transport.
	ChannelVKMiniApp Channel = "vk_miniapp"
	// ChannelWeb represents the standalone web platform transport.
	ChannelWeb Channel = "web"
)

// Valid reports whether c is a supported trusted transport.
func (c Channel) Valid() bool {
	switch c {
	case ChannelVKBot, ChannelVKMiniApp, ChannelWeb:
		return true
	default:
		return false
	}
}

// Publishable reports whether the platform currently has an external-push
// adapter for c. ChannelContext accepts every supported product surface, but
// an external delivery target must be limited to a transport that can actually
// publish a result. VK Mini App and web results are account-history only.
func (c Channel) Publishable() bool {
	return c == ChannelVKBot
}

// ChannelContext is bounded opaque origin provenance. It must never be used
// as an owner or authorization input; AccountID remains the canonical owner.
type ChannelContext struct {
	Channel      Channel `json:"channel"`
	RecipientRef string  `json:"recipient_ref,omitempty"`
	ThreadRef    string  `json:"thread_ref,omitempty"`
}

// Valid reports whether the context uses a supported channel and bounded
// optional opaque references.
func (c ChannelContext) Valid() bool {
	return c.Channel.Valid() && validOptionalChannelReference(c.RecipientRef) && validOptionalChannelReference(c.ThreadRef)
}

// DeliveryTarget is an explicit bounded external-push destination. It is
// distinct from ChannelContext so an origin cannot be mistaken for a target.
type DeliveryTarget struct {
	Channel      Channel `json:"channel"`
	RecipientRef string  `json:"recipient_ref"`
	ThreadRef    string  `json:"thread_ref,omitempty"`
}

// Valid reports whether a target is publishable by an installed channel
// adapter. A supported origin channel is not necessarily a push target.
func (t DeliveryTarget) Valid() bool {
	return t.Channel.Publishable() && validRequiredChannelReference(t.RecipientRef) && validOptionalChannelReference(t.ThreadRef)
}

// ResultMode controls how a ready job result is finalized. It is deliberately
// independent from the channel that created the job.
type ResultMode string

const (
	// ResultModeExternalPush publishes a result through a DeliveryTarget.
	ResultModeExternalPush ResultMode = "external_push"
	// ResultModeAccountHistory exposes a result through an owner-checked
	// account history/result boundary and creates no delivery attempt.
	ResultModeAccountHistory ResultMode = "account_history"
	// ResultModeLegacyUnknown is compatibility-only and must fail closed until
	// an explicit reconciliation assigns a concrete mode.
	ResultModeLegacyUnknown ResultMode = "legacy_unknown"
)

// Valid reports whether m is one of the persisted result modes.
func (m ResultMode) Valid() bool {
	switch m {
	case ResultModeExternalPush, ResultModeAccountHistory, ResultModeLegacyUnknown:
		return true
	default:
		return false
	}
}

// ErrInvalidResultContract marks a result-mode/channel-target shape that must
// never reach a finalizer.
var ErrInvalidResultContract = errors.New("domain: invalid result contract")

// ValidateResultContract verifies the persisted neutral delivery contract.
// Legacy-unknown records stay readable for compatibility, but callers must not
// treat them as an authorized publication instruction.
func (j Job) ValidateResultContract() error {
	if j.ChannelContext != nil && !j.ChannelContext.Valid() {
		return fmt.Errorf("%w: invalid channel context", ErrInvalidResultContract)
	}
	if j.DeliveryTarget != nil && !j.DeliveryTarget.Valid() {
		return fmt.Errorf("%w: invalid delivery target", ErrInvalidResultContract)
	}

	switch j.ResultMode {
	case ResultModeAccountHistory:
		if j.AccountID == [16]byte{} {
			return fmt.Errorf("%w: account history requires account owner", ErrInvalidResultContract)
		}
		if j.DeliveryTarget != nil {
			return fmt.Errorf("%w: account history must not have delivery target", ErrInvalidResultContract)
		}
		return nil
	case ResultModeExternalPush:
		if j.DeliveryTarget == nil {
			return fmt.Errorf("%w: external push requires delivery target", ErrInvalidResultContract)
		}
		return nil
	case ResultModeLegacyUnknown:
		if j.DeliveryTarget != nil {
			return fmt.Errorf("%w: legacy-unknown must not have delivery target", ErrInvalidResultContract)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported result mode %q", ErrInvalidResultContract, j.ResultMode)
	}
}

func validOptionalChannelReference(value string) bool {
	return value == "" || validRequiredChannelReference(value)
}

func validRequiredChannelReference(value string) bool {
	return len(value) <= MaxChannelReferenceLength && strings.TrimSpace(value) != ""
}
