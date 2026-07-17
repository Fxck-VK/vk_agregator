package domain

import "errors"

// ErrUnverifiedLogin is returned when an auth adapter tries to resolve an
// account before proving that the user controls the supplied identity.
var ErrUnverifiedLogin = errors.New("domain: unverified login")

// ErrAccountIdentityOwnershipRequired is returned when an account-linking
// operation cannot prove that the actor controls the account being modified.
var ErrAccountIdentityOwnershipRequired = errors.New("domain: account identity ownership required")

// ErrAccountLastIdentity is returned when unlinking an identity would leave an
// account without any usable login/channel binding.
var ErrAccountLastIdentity = errors.New("domain: account last identity cannot be unlinked")

// ErrAccountMergeRequiresConfirmation blocks implicit account merges. Merge is
// intentionally a separate audited flow with explicit user/operator consent.
var ErrAccountMergeRequiresConfirmation = errors.New("domain: account merge requires confirmation")

// AccountLoginMethod names a supported account login flow. Each method maps to
// an AccountIdentity provider after the method-specific proof is verified.
type AccountLoginMethod string

const (
	AccountLoginEmailPassword AccountLoginMethod = "email_password"
	AccountLoginPhoneOTP      AccountLoginMethod = "phone_otp"
	AccountLoginGoogle        AccountLoginMethod = "google"
	AccountLoginApple         AccountLoginMethod = "apple"
	AccountLoginTelegram      AccountLoginMethod = "telegram"
	AccountLoginVKID          AccountLoginMethod = "vk_id"
)

// VerifiedAccountLogin is the safe boundary between method-specific auth
// adapters and the account identity layer.
type VerifiedAccountLogin struct {
	Method     AccountLoginMethod
	ExternalID string
	Verified   bool
}
