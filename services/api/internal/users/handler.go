package users

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/email"
	"github.com/dag12y/devpulse/internal/oauth"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost balances verification latency against offline cracking
// resistance. Login and registration are low-frequency by design.
const bcryptCost = 12

const (
	maxRequestBodyBytes = 1 << 20
	// registerAttemptsPerHour caps account creation per IP. Registration
	// is open (every account gets its own isolated workspace), so abuse
	// prevention lives here, not in an invite system.
	registerAttemptsPerHour = 10
	// loginAttemptsPerHour slows online password guessing per IP on top
	// of bcrypt's per-attempt cost.
	loginAttemptsPerHour = 20
	// emailSendAttemptsPerHour caps verification/resend/forgot sends per
	// IP. These endpoints answer 202 regardless of whether an account
	// exists, so the limit is the only cost an address-harvesting script
	// pays for burning through inboxes.
	emailSendAttemptsPerHour = 6
	// resetAttemptsPerHour caps password-reset submissions per IP. The
	// token space is 256 bits, so this is belt-and-braces next to
	// single-use expiry.
	resetAttemptsPerHour = 10
	// oauthStartAttemptsPerHour caps authorize redirects per IP; each
	// start writes a state row and hands the browser to a third party.
	oauthStartAttemptsPerHour = 30
	authLimitWindow           = time.Hour
)

// Store is the persistence boundary for account handlers.
type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
	FindUserByEmail(ctx context.Context, email string) (*User, string, error)
	FindUserByID(ctx context.Context, userID string) (*User, error)
	CreateSession(ctx context.Context, userID string, token auth.GeneratedKey, expiresAt time.Time) error
	RevokeSession(ctx context.Context, tokenHash string) error
	CreateAuthToken(ctx context.Context, userID, kind, tokenHash string, expiresAt time.Time) error
	ConsumeAuthToken(ctx context.Context, kind, tokenHash string, now time.Time) (string, error)
	SetEmailVerified(ctx context.Context, userID string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	RevokeAllSessions(ctx context.Context, userID string) error
	CreateOAuthState(ctx context.Context, provider, stateHash, codeVerifier, redirectPath string, now, expiresAt time.Time) error
	ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (string, string, string, error)
	FindOAuthAccount(ctx context.Context, provider, providerUserID string) (string, error)
	LinkOAuthAccount(ctx context.Context, userID, provider, providerUserID, email string) error
	CreateWorkspace(ctx context.Context, userID, name string) (Membership, error)
	ListWorkspaces(ctx context.Context, userID string) ([]Membership, error)
	RenameWorkspace(ctx context.Context, userID, workspaceID, name string) (string, error)
	DeleteWorkspace(ctx context.Context, userID, workspaceID string) error
	FindMembership(ctx context.Context, userID, workspaceID string) (string, bool, error)
	ListMembers(ctx context.Context, workspaceID string) ([]Member, error)
	AddMember(ctx context.Context, workspaceID, email, role string) (*Member, error)
	UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) (*Member, error)
	RemoveMember(ctx context.Context, workspaceID, userID string) error
}

// Handler serves human account, workspace, and membership endpoints.
type Handler struct {
	store           Store
	now             func() time.Time
	registerLimiter *ipLimiter
	loginLimiter    *ipLimiter
	emailLimiter    *ipLimiter
	resetLimiter    *ipLimiter
	oauthLimiter    *ipLimiter
	// secureCookies switches the session cookie to the __Host- name with
	// the Secure attribute (production behind TLS).
	secureCookies bool
	// mailer delivers verification and reset links. Defaults to the dev
	// log sender so no configuration can silently drop auth mail.
	mailer email.Sender
	// appURL is the public dashboard origin links point back to.
	appURL string
	// oauthProviders holds the configured login providers (GitHub,
	// Google). Empty disables OAuth entirely.
	oauthProviders map[string]oauth.Provider
}

// Option customises handler behaviour. Production passes
// WithSecureCookies(true) so the session cookie is host-bound.
type Option func(*Handler)

func WithSecureCookies(secure bool) Option {
	return func(handler *Handler) {
		handler.secureCookies = secure
	}
}

// WithEmailSender selects the outbound delivery (Resend in production,
// the log sender in development).
func WithEmailSender(sender email.Sender) Option {
	return func(handler *Handler) {
		handler.mailer = sender
	}
}

// WithAppURL sets the origin verification/reset links are built from.
func WithAppURL(appURL string) Option {
	return func(handler *Handler) {
		handler.appURL = appURL
	}
}

// WithOAuthProviders registers the configured login providers. An empty
// map (the default) disables OAuth: the list endpoint reports nothing
// and start/callback answer 404.
func WithOAuthProviders(providers map[string]oauth.Provider) Option {
	return func(handler *Handler) {
		handler.oauthProviders = providers
	}
}

func NewHandler(store Store, options ...Option) *Handler {
	handler := &Handler{
		store:           store,
		now:             time.Now,
		registerLimiter: newIPLimiter(registerAttemptsPerHour, authLimitWindow),
		loginLimiter:    newIPLimiter(loginAttemptsPerHour, authLimitWindow),
		emailLimiter:    newIPLimiter(emailSendAttemptsPerHour, authLimitWindow),
		resetLimiter:    newIPLimiter(resetAttemptsPerHour, authLimitWindow),
		oauthLimiter:    newIPLimiter(oauthStartAttemptsPerHour, authLimitWindow),
		mailer:          email.NewLog(),
		appURL:          "http://localhost:3000",
		oauthProviders:  map[string]oauth.Provider{},
	}
	for _, option := range options {
		option(handler)
	}
	return handler
}

type registerInput struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	WorkspaceName string `json:"workspace_name"`
}

// Register creates a user, a personal workspace owned by them, and a
// pending email-verification token. Every account starts isolated: it
// can never see another workspace without an explicit membership. No
// session is issued yet — login refuses unverified accounts, so the
// emailed link is the front door.
func (handler *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !handler.registerLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many registration attempts")
		return
	}
	var input registerInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ValidatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workspaceName := input.WorkspaceName
	if workspaceName == "" {
		workspaceName = "My workspace"
	}
	workspaceName, err = ValidateWorkspaceName(workspaceName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		slog.Error("hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to register")
		return
	}
	user, err := handler.store.CreateUser(r.Context(), email, string(hash))
	if errors.Is(err, ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email is already registered")
		return
	}
	if err != nil {
		slog.Error("create user", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to register")
		return
	}
	membership, err := handler.store.CreateWorkspace(r.Context(), user.ID, workspaceName)
	if err != nil {
		slog.Error("create personal workspace", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to register")
		return
	}
	// The account exists either way; a failed mail only delays the user
	// (the verify page's resend button re-issues), so never fail the
	// signup itself.
	if err := handler.issueVerification(r.Context(), user.ID, user.Email); err != nil {
		slog.Error("issue verification token", "error", err)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user":                  user,
		"workspace":             membership,
		"verification_required": true,
	})
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login verifies credentials and issues a session token. Unknown emails
// and wrong passwords produce the identical response (and cost), so the
// endpoint does not reveal which emails are registered.
func (handler *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !handler.loginLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	var input loginInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	user, hash, err := handler.store.FindUserByEmail(r.Context(), email)
	if errors.Is(err, ErrNotFound) {
		burnUnknownUserAttempt(input.Password)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		slog.Error("find user", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to log in")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	// Checked after the password so the branch is only reachable by
	// someone who already proves account knowledge — no registration
	// oracle for wrong-password probes.
	if user.EmailVerifiedAt == nil {
		writeError(w, http.StatusForbidden, "email address not verified")
		return
	}
	token, expiresAt, err := handler.newSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("create login session", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to log in")
		return
	}
	workspaces, err := handler.store.ListWorkspaces(r.Context(), user.ID)
	if err != nil {
		slog.Error("list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to log in")
		return
	}
	handler.setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"workspaces": workspaces,
		"token":      token,
		"expires_at": expiresAt,
	})
}

// Logout revokes the current session. Unknown tokens still succeed so
// logout stays idempotent across retries and double submits.
func (handler *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	raw, ok := auth.BearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := handler.store.RevokeSession(r.Context(), auth.Hash(raw)); err != nil {
		slog.Error("revoke session", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to log out")
		return
	}
	handler.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type verifyEmailInput struct {
	Token string `json:"token"`
}

// VerifyEmail activates an account from the emailed link. The token is
// consumed atomically before the account flips: a link works exactly
// once, and unknown/expired/used tokens report the same error.
func (handler *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var input verifyEmailInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Token == "" {
		writeError(w, http.StatusBadRequest, "verification link is invalid or has expired")
		return
	}
	userID, err := handler.store.ConsumeAuthToken(r.Context(), TokenVerifyEmail, auth.Hash(input.Token), handler.now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusBadRequest, "verification link is invalid or has expired")
		return
	}
	if err != nil {
		slog.Error("consume verification token", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to verify email")
		return
	}
	if err := handler.store.SetEmailVerified(r.Context(), userID); err != nil {
		slog.Error("mark email verified", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to verify email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

type resendVerificationInput struct {
	Email string `json:"email"`
}

// ResendVerification re-issues the link for an unverified account. The
// 202 body is identical for unknown, verified, and unverified
// addresses so the endpoint never confirms which emails are registered.
func (handler *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if !handler.emailLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many email requests")
		return
	}
	var input resendVerificationInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, _, err := handler.store.FindUserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.Error("find user for verification resend", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to send verification email")
		return
	}
	if err == nil && user.EmailVerifiedAt == nil {
		if err := handler.issueVerification(r.Context(), user.ID, user.Email); err != nil {
			slog.Error("issue verification token", "error", err)
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

type forgotPasswordInput struct {
	Email string `json:"email"`
}

// ForgotPassword starts a reset by emailing a one-hour link. Like
// resend, the 202 answer never reveals whether the address exists.
func (handler *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !handler.emailLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many email requests")
		return
	}
	var input forgotPasswordInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, _, err := handler.store.FindUserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.Error("find user for password reset", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to send reset email")
		return
	}
	if err == nil {
		if err := handler.issuePasswordReset(r.Context(), user.ID, user.Email); err != nil {
			slog.Error("issue password reset token", "error", err)
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

type resetPasswordInput struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResetPassword consumes the emailed token, rotates the hash, verifies
// the email (inbox control is the same evidence verification asks for),
// and revokes every session the account holds.
func (handler *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !handler.resetLimiter.Allow(auth.ClientIP(r)) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many reset attempts")
		return
	}
	var input resetPasswordInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ValidatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Token == "" {
		writeError(w, http.StatusBadRequest, "reset link is invalid or has expired")
		return
	}
	userID, err := handler.store.ConsumeAuthToken(r.Context(), TokenPasswordReset, auth.Hash(input.Token), handler.now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusBadRequest, "reset link is invalid or has expired")
		return
	}
	if err != nil {
		slog.Error("consume password reset token", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to reset password")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		slog.Error("hash reset password", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to reset password")
		return
	}
	if err := handler.store.UpdatePassword(r.Context(), userID, string(hash)); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusBadRequest, "reset link is invalid or has expired")
			return
		}
		slog.Error("update password", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to reset password")
		return
	}
	// A revoke failure is logged, not surfaced: the password DID change,
	// and failing the response would claim otherwise while the
	// single-use token is already spent. Residual sessions die at
	// expiry; the log entry is the operator's cue.
	if err := handler.store.RevokeAllSessions(r.Context(), userID); err != nil {
		slog.Error("revoke sessions after password reset", "error", err, "user_id", userID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"password_updated": true})
}

// issueVerification stores a fresh verification token for the user and
// emails the link. Replaces any previous verification token.
func (handler *Handler) issueVerification(ctx context.Context, userID, address string) error {
	token, err := auth.GenerateToken()
	if err != nil {
		return err
	}
	expiresAt := handler.now().UTC().Add(verifyEmailTTL)
	if err := handler.store.CreateAuthToken(ctx, userID, TokenVerifyEmail, token.Hash, expiresAt); err != nil {
		return err
	}
	link := handler.appURL + "/verify-email?token=" + url.QueryEscape(token.Raw)
	handler.dispatchEmail(email.Message{
		To:      address,
		Subject: "Verify your email for DevPulse",
		Text: "Confirm your email address to activate your DevPulse account.\n\n" +
			"Open this link to verify:\n" + link + "\n\n" +
			"The link expires in 24 hours. If you didn't create a DevPulse account, you can ignore this email.",
		HTML: "<p>Confirm your email address to activate your DevPulse account.</p>" +
			`<p><a href="` + html.EscapeString(link) + `">Verify my email</a></p>` +
			"<p>The link expires in 24 hours. If you didn't create a DevPulse account, you can ignore this email.</p>",
	})
	return nil
}

// issuePasswordReset stores a one-hour reset token and emails the link.
// It replaces any previous reset token — and any pending verification
// link still works independently, since kinds never collide.
func (handler *Handler) issuePasswordReset(ctx context.Context, userID, address string) error {
	token, err := auth.GenerateToken()
	if err != nil {
		return err
	}
	expiresAt := handler.now().UTC().Add(passwordResetTTL)
	if err := handler.store.CreateAuthToken(ctx, userID, TokenPasswordReset, token.Hash, expiresAt); err != nil {
		return err
	}
	link := handler.appURL + "/reset-password?token=" + url.QueryEscape(token.Raw)
	handler.dispatchEmail(email.Message{
		To:      address,
		Subject: "Reset your DevPulse password",
		Text: "Someone asked to reset the password for this DevPulse account.\n\n" +
			"Open this link to choose a new password:\n" + link + "\n\n" +
			"The link expires in 1 hour. If you didn't request this, you can ignore this email — your password is unchanged.",
		HTML: "<p>Someone asked to reset the password for this DevPulse account.</p>" +
			`<p><a href="` + html.EscapeString(link) + `">Choose a new password</a></p>` +
			"<p>The link expires in 1 hour. If you didn't request this, you can ignore this email — your password is unchanged.</p>",
	})
	return nil
}

// dispatchEmail sends in the background. Auth mail is fire-and-forget:
// the HTTP answer must not depend on provider latency — a synchronous
// forgot-password call would reveal, by timing, which addresses are
// registered — and a slow provider must never stall a signup. Failures
// log; the resend button is the retry path.
func (handler *Handler) dispatchEmail(message email.Message) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := handler.mailer.Send(ctx, message); err != nil {
			slog.Error("send auth email", "error", err, "to", message.To, "subject", message.Subject)
		}
	}()
}

// Me returns the caller plus every workspace they belong to. It also
// slides the session's expiry forward (at most one renewal per
// renewalInterval), so active users stay signed in while abandoned
// sessions still expire on schedule.
func (handler *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	handler.touchSession(r.Context(), r)
	user, err := handler.store.FindUserByID(r.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	if err != nil {
		slog.Error("find user", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to load account")
		return
	}
	workspaces, err := handler.store.ListWorkspaces(r.Context(), userID)
	if err != nil {
		slog.Error("list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to load account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"workspaces": workspaces,
	})
}

type createWorkspaceInput struct {
	Name string `json:"name"`
}

// CreateWorkspace creates an additional workspace owned by the caller.
func (handler *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var input createWorkspaceInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := ValidateWorkspaceName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	membership, err := handler.store.CreateWorkspace(r.Context(), userID, name)
	if errors.Is(err, ErrNameTaken) {
		writeError(w, http.StatusConflict, "workspace name is already in use")
		return
	}
	if err != nil {
		slog.Error("create workspace", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to create workspace")
		return
	}
	writeJSON(w, http.StatusCreated, membership)
}

// ListWorkspaces returns every workspace the caller belongs to.
func (handler *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	workspaces, err := handler.store.ListWorkspaces(r.Context(), userID)
	if err != nil {
		slog.Error("list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to list workspaces")
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
}

type renameWorkspaceInput struct {
	Name string `json:"name"`
}

// RenameWorkspace renames a workspace. Owner/admin only; viewers receive
// 403. Non-members (and unknown IDs) report 404 so workspace existence is
// never leaked; a per-user duplicate name reports 409.
func (handler *Handler) RenameWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	userID, _, ok := handler.membership(w, r, workspaceID, true)
	if !ok {
		return
	}
	var input renameWorkspaceInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := ValidateWorkspaceName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	renamed, err := handler.store.RenameWorkspace(r.Context(), userID, workspaceID, name)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if errors.Is(err, ErrNameTaken) {
		writeError(w, http.StatusConflict, "workspace name is already in use")
		return
	}
	if err != nil {
		slog.Error("rename workspace", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to rename workspace")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"workspace_id": workspaceID, "workspace_name": renamed})
}

// DeleteWorkspace deletes the workspace and everything it owns (projects,
// API keys, memberships cascade). Owner only — admins receive 403 —
// because deletion is irreversible and wipes analytics data. Deleting the
// caller's last workspace is allowed: ListWorkspaces then returns empty
// and the dashboard shows the create-workspace state.
func (handler *Handler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	userID, role, ok := handler.membership(w, r, workspaceID, false)
	if !ok {
		return
	}
	if role != auth.RoleOwner {
		writeError(w, http.StatusForbidden, "only workspace owners can delete the workspace")
		return
	}
	if err := handler.store.DeleteWorkspace(r.Context(), userID, workspaceID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		slog.Error("delete workspace", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to delete workspace")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns the workspace roster. Membership itself is the
// authorization: non-members get 404, never a roster-shaped 403.
func (handler *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	if _, _, ok := handler.membership(w, r, workspaceID, false); !ok {
		return
	}
	members, err := handler.store.ListMembers(r.Context(), workspaceID)
	if err != nil {
		slog.Error("list members", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to list members")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

type addMemberInput struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AddMember adds an already-registered user to the workspace.
// Owner/admin only; viewers receive 403.
func (handler *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	if _, _, ok := handler.membership(w, r, workspaceID, true); !ok {
		return
	}
	var input addMemberInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	role := input.Role
	if role == "" {
		role = auth.RoleViewer
	}
	if role, err = ValidateRole(role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	member, err := handler.store.AddMember(r.Context(), workspaceID, email, role)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "user is not registered")
		return
	}
	if errors.Is(err, ErrAlreadyMember) {
		writeError(w, http.StatusConflict, "user is already a member of this workspace")
		return
	}
	if err != nil {
		slog.Error("add member", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to add member")
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

type updateMemberInput struct {
	Role string `json:"role"`
}

// UpdateMemberRole changes a member's role. Owner/admin only; demoting
// the last owner reports 409.
func (handler *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	if _, _, ok := handler.membership(w, r, workspaceID, true); !ok {
		return
	}
	targetID, ok := memberPathID(w, r)
	if !ok {
		return
	}
	var input updateMemberInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	role, err := ValidateRole(input.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	member, err := handler.store.UpdateMemberRole(r.Context(), workspaceID, targetID, role)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	if errors.Is(err, ErrLastOwner) {
		writeError(w, http.StatusConflict, "workspace must keep at least one owner")
		return
	}
	if err != nil {
		slog.Error("update member", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to update member")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

// RemoveMember removes a member. Owners/admins may remove anyone except
// the last owner; any member may remove themselves (leave), guarded by
// the same last-owner rule.
func (handler *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := workspacePathID(w, r)
	if !ok {
		return
	}
	callerID, _, ok := handler.membership(w, r, workspaceID, false)
	if !ok {
		return
	}
	targetID, ok := memberPathID(w, r)
	if !ok {
		return
	}
	if targetID != callerID {
		if _, _, ok := handler.membership(w, r, workspaceID, true); !ok {
			return
		}
	}
	if err := handler.store.RemoveMember(r.Context(), workspaceID, targetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}
		if errors.Is(err, ErrLastOwner) {
			writeError(w, http.StatusConflict, "workspace must keep at least one owner")
			return
		}
		slog.Error("remove member", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// membership resolves the caller's role and enforces write access when
// needed. Unknown workspaces and non-membership both report 404 so
// workspace existence is never leaked.
func (handler *Handler) membership(w http.ResponseWriter, r *http.Request, workspaceID string, needWrite bool) (userID, role string, ok bool) {
	callerID, present := auth.UserFromContext(r.Context())
	if !present {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return "", "", false
	}
	userID = callerID
	role, member, err := handler.store.FindMembership(r.Context(), userID, workspaceID)
	if err != nil {
		slog.Error("find membership", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to authorize")
		return "", "", false
	}
	if !member {
		writeError(w, http.StatusNotFound, "workspace not found")
		return "", "", false
	}
	if needWrite && !auth.CanWrite(role) {
		writeError(w, http.StatusForbidden, "viewer role cannot modify resources")
		return "", "", false
	}
	return userID, role, true
}

func (handler *Handler) newSession(ctx context.Context, userID string) (raw string, expiresAt time.Time, err error) {
	token, err := auth.GenerateSession()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = handler.now().UTC().Add(SessionLifetime)
	if err := handler.store.CreateSession(ctx, userID, token, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token.Raw, expiresAt, nil
}

// sessionToucher slides a live session's expiry. Optional: stores that
// implement it get sliding renewal, others keep fixed lifetimes.
type sessionToucher interface {
	TouchSession(ctx context.Context, tokenHash string, now time.Time) error
}

// touchSession renews the caller's session best-effort: a failed renewal
// never fails the request that triggered it, since the session has just
// been authenticated as valid.
func (handler *Handler) touchSession(ctx context.Context, r *http.Request) {
	toucher, ok := handler.store.(sessionToucher)
	if !ok {
		return
	}
	raw, ok := auth.BearerToken(r)
	if !ok {
		return
	}
	if err := toucher.TouchSession(ctx, auth.Hash(raw), handler.now().UTC()); err != nil {
		slog.Warn("slide session expiry", "error", err)
	}
}

// burnUnknownUserAttempt spends bcrypt work on the unknown-email path so
// login timing does not reveal which emails are registered.
func burnUnknownUserAttempt(password string) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("devpulse-unknown-user-dummy"), bcryptCost)
	if err != nil {
		return
	}
	_ = bcrypt.CompareHashAndPassword(dummy, []byte(password))
}

func workspacePathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	return pathUUID(w, r, "id", "workspace id must be a UUID")
}

func memberPathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	return pathUUID(w, r, "userId", "member id must be a UUID")
}

func pathUUID(w http.ResponseWriter, r *http.Request, name, message string) (string, bool) {
	id := r.PathValue(name)
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, message)
		return "", false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("request body must be valid JSON")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write users JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// ipLimiter is a fixed-window limiter keyed by IP for auth endpoints.
// In-memory and per-instance, like the ingestion limiter.
type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
	limit   int
	window  time.Duration
}

type ipBucket struct {
	count       int
	windowStart time.Time
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{buckets: map[string]*ipBucket{}, limit: limit, window: window}
}

func (limiter *ipLimiter) Allow(ip string) bool {
	now := time.Now().UTC()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	bucket, ok := limiter.buckets[ip]
	if !ok || now.Sub(bucket.windowStart) >= limiter.window {
		limiter.buckets[ip] = &ipBucket{count: 1, windowStart: now}
		return true
	}
	bucket.count++
	return bucket.count <= limiter.limit
}
