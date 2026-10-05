package users

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// SessionLifetime bounds how long a login stays valid without
	// re-authenticating. There is no sliding renewal: callers log in again.
	SessionLifetime = 30 * 24 * time.Hour

	minPasswordLength = 12
	// bcrypt silently truncates past 72 bytes; reject longer passwords
	// instead of pretending the tail was verified.
	maxPasswordBytes = 72
	maxEmailLength   = 254
	maxNameLength    = 255
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
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// User is the public account shape. Password hashes never leave the store.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
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
