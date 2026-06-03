package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role describes the access level a user has within the platform. It governs
// whether a user can only consume generation features or also reach the admin
// control plane.
type Role string

const (
	// RoleUser is a regular end user interacting through VK.
	RoleUser Role = "user"
	// RoleModerator can review moderation queues and take manual decisions.
	RoleModerator Role = "moderator"
	// RoleAdmin has full access to the admin control plane.
	RoleAdmin Role = "admin"
)

// Valid reports whether the role is one of the known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleModerator, RoleAdmin:
		return true
	default:
		return false
	}
}

// Status is the lifecycle state of a user account. It is used for anti-spam,
// banning and soft-deletion flows.
type Status string

const (
	// StatusActive is a normal account allowed to create jobs.
	StatusActive Status = "active"
	// StatusBlocked is temporarily blocked, usually by anti-spam.
	StatusBlocked Status = "blocked"
	// StatusBanned is permanently banned by an administrator.
	StatusBanned Status = "banned"
	// StatusDeleted is soft-deleted and retained only for audit.
	StatusDeleted Status = "deleted"
)

// Valid reports whether the status is one of the known statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusBlocked, StatusBanned, StatusDeleted:
		return true
	default:
		return false
	}
}

// User is the canonical identity of a VK user known to the platform. It is the
// owner of jobs, artifacts, deliveries and billing accounts.
type User struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// VKUserID is the external VK identifier. It is unique per user.
	VKUserID int64 `json:"vk_user_id"`
	// Role controls the access level of the user.
	Role Role `json:"role"`
	// Status is the lifecycle state of the account.
	Status Status `json:"status"`
	// Locale is the preferred IETF language tag, e.g. "ru" or "en".
	Locale string `json:"locale"`
	// Timezone is the IANA timezone name, e.g. "Europe/Moscow".
	Timezone string `json:"timezone"`
	// RiskLevel is an anti-abuse score from 0 (trusted) to 100 (high risk).
	RiskLevel int `json:"risk_level"`
	// FirstSeenAt is when the platform first observed the user.
	FirstSeenAt time.Time `json:"first_seen_at"`
	// LastSeenAt is when the user last interacted with the platform.
	LastSeenAt time.Time `json:"last_seen_at"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the last mutation timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// IsActive reports whether the user is allowed to create new jobs.
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}
