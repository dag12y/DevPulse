package users

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
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
	authLimitWindow      = time.Hour
)

// Store is the persistence boundary for account handlers.
type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
	FindUserByEmail(ctx context.Context, email string) (*User, string, error)
	FindUserByID(ctx context.Context, userID string) (*User, error)
	CreateSession(ctx context.Context, userID string, token auth.GeneratedKey, expiresAt time.Time) error
	RevokeSession(ctx context.Context, tokenHash string) error
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
	// secureCookies switches the session cookie to the __Host- name with
	// the Secure attribute (production behind TLS).
	secureCookies bool
}

// Option customises handler behaviour. Production passes
// WithSecureCookies(true) so the session cookie is host-bound.
type Option func(*Handler)

func WithSecureCookies(secure bool) Option {
	return func(handler *Handler) {
		handler.secureCookies = secure
	}
}

func NewHandler(store Store, options ...Option) *Handler {
	handler := &Handler{
		store:           store,
		now:             time.Now,
		registerLimiter: newIPLimiter(registerAttemptsPerHour, authLimitWindow),
		loginLimiter:    newIPLimiter(loginAttemptsPerHour, authLimitWindow),
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

// Register creates a user, a personal workspace owned by them, and a first
// session. Every account starts isolated: it can never see another
// workspace without an explicit membership.
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
	token, expiresAt, err := handler.newSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("create registration session", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to register")
		return
	}
	handler.setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusCreated, map[string]any{
		"user":       user,
		"workspace":  membership,
		"token":      token,
		"expires_at": expiresAt,
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
