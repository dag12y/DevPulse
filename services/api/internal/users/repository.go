package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists users, sessions, workspaces, and memberships.
// Password hashes are BYTEA-safe TEXT (bcrypt strings); only hashes cross
// this boundary, never raw passwords or tokens.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreateUser inserts a user with an already-hashed password. Accounts
// start unverified; login refuses them until the emailed link flips
// email_verified_at.
func (repository *Repository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	user := new(User)
	err := repository.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id::text, email, email_verified_at, created_at`,
		email, passwordHash).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// FindUserByEmail returns the user plus password hash for verification.
func (repository *Repository) FindUserByEmail(ctx context.Context, email string) (*User, string, error) {
	user := new(User)
	var hash string
	err := repository.pool.QueryRow(ctx,
		`SELECT id::text, email, email_verified_at, password_hash, created_at FROM users WHERE email = $1`,
		email).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &hash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("find user: %w", err)
	}
	return user, hash, nil
}

// FindUserByID returns a user profile by ID.
func (repository *Repository) FindUserByID(ctx context.Context, userID string) (*User, error) {
	user := new(User)
	err := repository.pool.QueryRow(ctx,
		`SELECT id::text, email, email_verified_at, created_at FROM users WHERE id = $1`,
		userID).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

// CreateSession persists a hashed login token. The raw token is shown to
// the caller exactly once and never stored.
func (repository *Repository) CreateSession(ctx context.Context, userID string, token auth.GeneratedKey, expiresAt time.Time) error {
	_, err := repository.pool.Exec(ctx,
		`INSERT INTO user_sessions (user_id, token_hash, token_prefix, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, token.Hash, token.Prefix, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// FindSession implements auth.SessionStore: unknown, revoked, or expired
// tokens all report ok=false without distinction.
func (repository *Repository) FindSession(ctx context.Context, tokenHash string, now time.Time) (string, bool, error) {
	var userID string
	err := repository.pool.QueryRow(ctx,
		`SELECT user_id::text FROM user_sessions
		  WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2`,
		tokenHash, now).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("find session: %w", err)
	}
	return userID, true, nil
}

// RevokeSession logs out one session; unknown tokens are a no-op success
// so logout stays idempotent.
func (repository *Repository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := repository.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = NOW()
		  WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// TouchSession slides a live session's expiry to now + SessionLifetime.
// The guard clause keeps renewals to at most one per renewalInterval even
// under constant traffic, and never extends revoked or expired sessions
// (those must keep their original expiry so retention can purge them).
func (repository *Repository) TouchSession(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := repository.pool.Exec(ctx,
		`UPDATE user_sessions SET expires_at = $1
		  WHERE token_hash = $2 AND revoked_at IS NULL
		    AND expires_at > $3 AND expires_at <= $4`,
		now.Add(SessionLifetime), tokenHash, now, now.Add(SessionLifetime-renewalInterval))
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// CreateAuthToken stores a hashed email token, replacing any previous
// token of the same kind for the same user: at most one live link per
// purpose, so an old mail can never out-race a newer one.
func (repository *Repository) CreateAuthToken(ctx context.Context, userID, kind, tokenHash string, expiresAt time.Time) error {
	_, err := repository.pool.Exec(ctx,
		`WITH replaced AS (
			DELETE FROM auth_tokens WHERE user_id = $1 AND kind = $2
		 )
		 INSERT INTO auth_tokens (user_id, kind, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, kind, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}
	return nil
}

// ConsumeAuthToken atomically deletes a live token and returns its
// owner. Unknown, expired, already-used, and wrong-kind tokens all
// report ErrNotFound without distinction — links are single-use, and
// the response must not say which of those happened.
func (repository *Repository) ConsumeAuthToken(ctx context.Context, kind, tokenHash string, now time.Time) (string, error) {
	var userID string
	err := repository.pool.QueryRow(ctx,
		`DELETE FROM auth_tokens
		  WHERE token_hash = $1 AND kind = $2 AND expires_at > $3
		  RETURNING user_id::text`,
		tokenHash, kind, now).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("consume auth token: %w", err)
	}
	return userID, nil
}

// SetEmailVerified marks an account verified. COALESCE keeps the first
// verification time rather than overwriting it on re-verification.
func (repository *Repository) SetEmailVerified(ctx context.Context, userID string) error {
	tag, err := repository.pool.Exec(ctx,
		`UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()), updated_at = NOW()
		  WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePassword rotates the password hash. It also verifies the email:
// proving control of the inbox (a consumed reset link) is exactly the
// evidence verification asks for.
func (repository *Repository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	tag, err := repository.pool.Exec(ctx,
		`UPDATE users
		  SET password_hash = $2,
		      email_verified_at = COALESCE(email_verified_at, NOW()),
		      updated_at = NOW()
		  WHERE id = $1`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAllSessions logs out every device. Password changes call it:
// whoever requested the reset must not inherit an attacker's existing
// sessions, and the legitimate owner's other sessions die too (they can
// sign in again with the new password).
func (repository *Repository) RevokeAllSessions(ctx context.Context, userID string) error {
	_, err := repository.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = NOW()
		  WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

// CreateOAuthState stores a hashed login state with its PKCE verifier,
// purging expired states in the same statement so the table cannot
// grow without bound between logins.
func (repository *Repository) CreateOAuthState(ctx context.Context, provider, stateHash, codeVerifier, redirectPath string, now, expiresAt time.Time) error {
	_, err := repository.pool.Exec(ctx,
		`WITH purged AS (
			DELETE FROM oauth_states WHERE expires_at < $1
		 )
		 INSERT INTO oauth_states (state_hash, code_verifier, provider, redirect_path, expires_at)
		 VALUES ($2, $3, $4, $5, $6)`,
		now, stateHash, codeVerifier, provider, redirectPath, expiresAt)
	if err != nil {
		return fmt.Errorf("create oauth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState atomically deletes a live state and returns its
// login context. Unknown and expired states report ErrNotFound — the
// answer is identical for a forged state and a slow one.
func (repository *Repository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (string, string, string, error) {
	var provider, codeVerifier, redirectPath string
	err := repository.pool.QueryRow(ctx,
		`DELETE FROM oauth_states
		  WHERE state_hash = $1 AND expires_at > $2
		  RETURNING provider, code_verifier, COALESCE(redirect_path, '')`,
		stateHash, now).Scan(&provider, &codeVerifier, &redirectPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrNotFound
		}
		return "", "", "", fmt.Errorf("consume oauth state: %w", err)
	}
	return provider, codeVerifier, redirectPath, nil
}

// FindOAuthAccount resolves a provider identity to a local user.
func (repository *Repository) FindOAuthAccount(ctx context.Context, provider, providerUserID string) (string, error) {
	var userID string
	err := repository.pool.QueryRow(ctx,
		`SELECT user_id::text FROM oauth_accounts
		  WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("find oauth account: %w", err)
	}
	return userID, nil
}

// LinkOAuthAccount binds a provider identity to a user. A uniqueness
// conflict means the identity already belongs to another account —
// refuse rather than steal the binding.
func (repository *Repository) LinkOAuthAccount(ctx context.Context, userID, provider, providerUserID, email string) error {
	_, err := repository.pool.Exec(ctx,
		`INSERT INTO oauth_accounts (user_id, provider, provider_user_id, email)
		 VALUES ($1, $2, $3, $4)`,
		userID, provider, providerUserID, email)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrOAuthConflict
		}
		return fmt.Errorf("link oauth account: %w", err)
	}
	return nil
}

// nameTakenByUser reports whether the user already belongs to a workspace
// with this name (case-insensitive), optionally excluding one workspace
// (the rename target itself).
func (repository *Repository) nameTakenByUser(ctx context.Context, userID, name, excludeWorkspaceID string) (bool, error) {
	var exists bool
	if err := repository.pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		   WHERE m.user_id = $1 AND lower(w.name) = lower($2)
		     AND ($3 = '' OR m.workspace_id <> $3::uuid)
		 )`,
		userID, name, excludeWorkspaceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check workspace name: %w", err)
	}
	return exists, nil
}

// CreateWorkspace creates a workspace with the caller as owner.
func (repository *Repository) CreateWorkspace(ctx context.Context, userID, name string) (Membership, error) {
	taken, err := repository.nameTakenByUser(ctx, userID, name, "")
	if err != nil {
		return Membership{}, err
	}
	if taken {
		return Membership{}, ErrNameTaken
	}
	var membership Membership
	err = repository.pool.QueryRow(ctx,
		`WITH workspace AS (
			INSERT INTO workspaces (name) VALUES ($2) RETURNING id, name
		), member AS (
			INSERT INTO workspace_members (workspace_id, user_id, role)
			SELECT id, $1, 'owner' FROM workspace RETURNING workspace_id
		)
		SELECT workspace.id::text, workspace.name FROM workspace`,
		userID, name).Scan(&membership.WorkspaceID, &membership.WorkspaceName)
	if err != nil {
		return membership, fmt.Errorf("create workspace: %w", err)
	}
	membership.Role = auth.RoleOwner
	return membership, nil
}

// RenameWorkspace renames a workspace the caller belongs to. Ownership is
// checked by the handler (owner/admin only); here we enforce membership,
// per-user name uniqueness, and return the updated name.
func (repository *Repository) RenameWorkspace(ctx context.Context, userID, workspaceID, name string) (string, error) {
	var member bool
	if err := repository.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		workspaceID, userID).Scan(&member); err != nil {
		return "", fmt.Errorf("check membership: %w", err)
	}
	if !member {
		return "", ErrNotFound
	}
	taken, err := repository.nameTakenByUser(ctx, userID, name, workspaceID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", ErrNameTaken
	}
	var renamed string
	if err := repository.pool.QueryRow(ctx,
		`UPDATE workspaces SET name = $2, updated_at = NOW()
		  WHERE id = $1 RETURNING name`,
		workspaceID, name).Scan(&renamed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("rename workspace: %w", err)
	}
	return renamed, nil
}

// DeleteWorkspace removes the workspace and everything it owns. All child
// tables (analytics_projects, workspace_api_keys, workspace_members)
// cascade from workspaces(id), so one DELETE is the whole operation.
func (repository *Repository) DeleteWorkspace(ctx context.Context, userID, workspaceID string) error {
	var member bool
	if err := repository.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		workspaceID, userID).Scan(&member); err != nil {
		return fmt.Errorf("check membership: %w", err)
	}
	if !member {
		return ErrNotFound
	}
	tag, err := repository.pool.Exec(ctx,
		`DELETE FROM workspaces WHERE id = $1`, workspaceID)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListWorkspaces returns every workspace the user belongs to, ordered by
// creation time with the workspace ID as a tiebreaker so listing is
// deterministic even when two rows share a timestamp.
func (repository *Repository) ListWorkspaces(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := repository.pool.Query(ctx,
		`SELECT m.workspace_id::text, w.name, m.role
		  FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		  WHERE m.user_id = $1 ORDER BY w.created_at, w.id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()
	memberships := make([]Membership, 0)
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(&membership.WorkspaceID, &membership.WorkspaceName, &membership.Role); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	return memberships, nil
}

// FindMembership returns the caller's role in a workspace.
func (repository *Repository) FindMembership(ctx context.Context, userID, workspaceID string) (string, bool, error) {
	var role string
	err := repository.pool.QueryRow(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("find membership: %w", err)
	}
	return role, true, nil
}

// ListMembers returns the workspace roster. Callers must already hold
// membership (enforced by handlers, not here).
func (repository *Repository) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	rows, err := repository.pool.Query(ctx,
		`SELECT m.user_id::text, u.email, m.role
		  FROM workspace_members m JOIN users u ON u.id = m.user_id
		  WHERE m.workspace_id = $1 ORDER BY u.email`,
		workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.UserID, &member.Email, &member.Role); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return members, nil
}

// AddMember adds an existing user by email. Invites to unknown emails
// report ErrNotFound so callers can distinguish "not registered".
func (repository *Repository) AddMember(ctx context.Context, workspaceID, email, role string) (*Member, error) {
	var member Member
	err := repository.pool.QueryRow(ctx,
		`WITH target AS (SELECT id FROM users WHERE email = $2)
		INSERT INTO workspace_members (workspace_id, user_id, role)
		SELECT $1, target.id, $3 FROM target
		RETURNING user_id::text, (SELECT email FROM users WHERE id = user_id), role`,
		workspaceID, email, role).Scan(&member.UserID, &member.Email, &member.Role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return nil, ErrAlreadyMember
		}
		// A bogus workspace_id fails the FK as a foreign-key violation.
		if errors.As(err, &postgresError) && postgresError.Code == "23503" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("add member: %w", err)
	}
	return &member, nil
}

// UpdateMemberRole changes a member's role, refusing to demote the last owner.
func (repository *Repository) UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) (*Member, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin role update: %w", err)
	}
	defer tx.Rollback(ctx)

	var current string
	if err := tx.QueryRow(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find member: %w", err)
	}
	if current == auth.RoleOwner && role != auth.RoleOwner && !repository.hasOtherOwner(ctx, tx, workspaceID, userID) {
		return nil, ErrLastOwner
	}
	member := new(Member)
	if err := tx.QueryRow(ctx,
		`UPDATE workspace_members SET role = $3
		  WHERE workspace_id = $1 AND user_id = $2
		  RETURNING user_id::text, (SELECT email FROM users WHERE id = user_id), role`,
		workspaceID, userID, role).Scan(&member.UserID, &member.Email, &member.Role); err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit role update: %w", err)
	}
	return member, nil
}

// RemoveMember removes a member, refusing to remove the last owner.
func (repository *Repository) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin member removal: %w", err)
	}
	defer tx.Rollback(ctx)

	var current string
	if err := tx.QueryRow(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("find member: %w", err)
	}
	if current == auth.RoleOwner && !repository.hasOtherOwner(ctx, tx, workspaceID, userID) {
		return ErrLastOwner
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit member removal: %w", err)
	}
	return nil
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (repository *Repository) hasOtherOwner(ctx context.Context, db querier, workspaceID, excludeUserID string) bool {
	var exists bool
	_ = db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace_members
		  WHERE workspace_id = $1 AND role = 'owner' AND user_id <> $2)`,
		workspaceID, excludeUserID).Scan(&exists)
	return exists
}
