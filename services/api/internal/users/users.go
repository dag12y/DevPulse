package users

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// SessionLifetime bounds how long a login stays valid. Sessions slide:
	// each /v1/auth/me call extends expiry to now + SessionLifetime, but
	// the WHERE clause in TouchSession caps that at one renewal per
	// renewalInterval, so a busy client cannot write on every request.
	SessionLifetime = 30 * 24 * time.Hour
	// renewalInterval is the minimum gap between two sliding renewals of
	// the same session.
	renewalInterval = time.Hour

	minPasswordLength = 12
	// bcrypt silently truncates past 72 bytes; reject longer passwords
	// instead of pretending the tail was verified.
	maxPasswordBytes = 72
	maxEmailLength   = 254
	maxNameLength    = 255

	// TokenTTL bounds how long an emailed link stays valid. Verification
	// gets a day (inbox digests, mobile clients); password reset gets an
	// hour because it immediately unlocks account takeover.
	verifyEmailTTL   = 24 * time.Hour
	passwordResetTTL = time.Hour
)

// Auth token kinds. One live token per (user, kind): issuing replaces
// the previous row, consuming deletes it.
const (
	TokenVerifyEmail   = "verify_email"
	TokenPasswordReset = "password_reset"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrEmailTaken    = errors.New("email is already registered")
	ErrAlreadyMember = errors.New("user is already a member of this workspace")
	ErrLastOwner     = errors.New("workspace must keep at least one owner")
	ErrForbidden     = errors.New("forbidden")
	// ErrNameTaken reports a per-user duplicate workspace name. Names are
	// unique per user (case-insensitive), not globally: two strangers may
	// both own "Acme", but one user may not own it twice — identical rows
	// are indistinguishable in the workspace switcher.
	ErrNameTaken = errors.New("workspace name is already in use")
	// ErrOAuthConflict reports an OAuth identity already bound to a
	// different user. Refusing the link is the only safe answer —
	// rebinding would let the second claimant take over the first
	// account.
	ErrOAuthConflict = errors.New("oauth identity is already linked to another account")
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// User is the public account shape. Password hashes and TOTP secrets
// never leave the store.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// EmailVerifiedAt is null until the owner has clicked the emailed
	// verification link; login refuses unverified accounts.
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	// TOTPEnabled reports whether two-factor auth is active. The pending
	// secret (set by setup, not yet verified) is never exposed here.
	TOTPEnabled bool      `json:"totp_enabled,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SessionInfo is one row of the "where am I signed in" list. The token
// prefix (first 12 characters, e.g. "dps_ab12cd34ef56") is safe to
// display: it identifies the row without being the credential.
type SessionInfo struct {
	ID          string     `json:"id"`
	TokenPrefix string     `json:"token_prefix"`
	IP          string     `json:"ip"`
	UserAgent   string     `json:"user_agent"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Current     bool       `json:"current"`
}

// Membership pairs a workspace with the caller's role in it.
type Membership struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Role          string `json:"role"`
}

// Member is a workspace roster entry.
type Member struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// NormalizeEmail trims and lowercases. Emails are stored lowercased with a
// database CHECK enforcing the invariant, so lookups are exact matches.
func NormalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", errors.New("email is required")
	}
	if len(normalized) > maxEmailLength {
		return "", fmt.Errorf("email must not exceed %d characters", maxEmailLength)
	}
	if !emailPattern.MatchString(normalized) {
		return "", errors.New("email must be a valid email address")
	}
	return normalized, nil
}

// ValidatePassword enforces a minimum length (and bcrypt's byte ceiling).
// No complexity rules: length plus bcrypt is the requirement.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("password must not exceed %d bytes", maxPasswordBytes)
	}
	return nil
}

// ValidateWorkspaceName mirrors project naming rules.
func ValidateWorkspaceName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("workspace name is required")
	}
	if len(trimmed) > maxNameLength {
		return "", fmt.Errorf("workspace name must not exceed %d characters", maxNameLength)
	}
	return trimmed, nil
}

// ValidateRole accepts only known workspace roles.
func ValidateRole(role string) (string, error) {
	switch role {
	case "owner", "admin", "viewer":
		return role, nil
	default:
		return "", errors.New("role must be one of owner, admin, or viewer")
	}
}
