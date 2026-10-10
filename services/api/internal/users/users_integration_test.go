package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/database"
	"github.com/dag12y/devpulse/internal/oauth"
	"github.com/dag12y/devpulse/internal/totp"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://devpulse:devpulse_dev_password@localhost:5433/devpulse_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := database.Migrate(context.Background(), pool); err != nil {
		pool.Close()
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func hashForTest(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

// TestUserLifecycle exercises register/login/session/member flows against
// Postgres: uniqueness, roles, last-owner guards, expiry, revocation,
// and cross-workspace denial.
func TestUserLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	handler := NewHandler(repository)

	var userIDs, workspaceIDs []string
	defer func() {
		for _, id := range workspaceIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, id)
		}
		for _, id := range userIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		}
	}()

	alice, err := repository.CreateUser(ctx, "alice@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, alice.ID)

	if _, err := repository.CreateUser(ctx, "alice@example.com", hashForTest(t, "correct-horse-12")); err != ErrEmailTaken {
		t.Fatalf("duplicate email err = %v, want ErrEmailTaken", err)
	}

	membership, err := repository.CreateWorkspace(ctx, alice.ID, "Alice Co")
	if err != nil {
		t.Fatal(err)
	}
	workspaceIDs = append(workspaceIDs, membership.WorkspaceID)
	if membership.Role != auth.RoleOwner {
		t.Fatalf("creator role = %q, want owner", membership.Role)
	}

	bob, err := repository.CreateUser(ctx, "bob@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, bob.ID)
	// Login refuses unverified accounts; verify Bob the way the emailed
	// link would (SetEmailVerified is the endpoint's store call).
	if err := repository.SetEmailVerified(ctx, bob.ID); err != nil {
		t.Fatal(err)
	}

	added, err := repository.AddMember(ctx, membership.WorkspaceID, "bob@example.com", auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if added.UserID != bob.ID || added.Role != auth.RoleViewer {
		t.Fatalf("member = %+v", added)
	}
	if _, err := repository.AddMember(ctx, membership.WorkspaceID, "bob@example.com", auth.RoleViewer); err != ErrAlreadyMember {
		t.Fatalf("double add err = %v, want ErrAlreadyMember", err)
	}
	if _, err := repository.AddMember(ctx, membership.WorkspaceID, "ghost@example.com", auth.RoleViewer); err != ErrNotFound {
		t.Fatalf("unknown email err = %v, want ErrNotFound", err)
	}

	// Sole owner cannot be demoted or removed.
	if _, err := repository.UpdateMemberRole(ctx, membership.WorkspaceID, alice.ID, auth.RoleAdmin); err != ErrLastOwner {
		t.Fatalf("demote last owner err = %v, want ErrLastOwner", err)
	}
	if err := repository.RemoveMember(ctx, membership.WorkspaceID, alice.ID); err != ErrLastOwner {
		t.Fatalf("remove last owner err = %v, want ErrLastOwner", err)
	}

	// Promote Bob, then Alice may leave.
	if _, err := repository.UpdateMemberRole(ctx, membership.WorkspaceID, bob.ID, auth.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if err := repository.RemoveMember(ctx, membership.WorkspaceID, alice.ID); err != nil {
		t.Fatal(err)
	}
	members, err := repository.ListMembers(ctx, membership.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].UserID != bob.ID || members[0].Role != auth.RoleOwner {
		t.Fatalf("members = %+v", members)
	}

	// Stranger Carol gets her own workspace and has no access to Alice's.
	carol, err := repository.CreateUser(ctx, "carol@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, carol.ID)
	carolMembership, err := repository.CreateWorkspace(ctx, carol.ID, "Carol Co")
	if err != nil {
		t.Fatal(err)
	}
	workspaceIDs = append(workspaceIDs, carolMembership.WorkspaceID)
	if _, member, err := repository.FindMembership(ctx, carol.ID, membership.WorkspaceID); err != nil || member {
		t.Fatalf("carol membership in alice's workspace = %v, %v; want false, nil", member, err)
	}

	// Sessions: valid, expired, and revoked tokens.
	now := time.Now().UTC()
	token, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSession(ctx, bob.ID, token, now.Add(time.Hour), "203.0.113.7", "test-agent"); err != nil {
		t.Fatal(err)
	}
	if userID, ok, err := repository.FindSession(ctx, token.Hash, now); err != nil || !ok || userID != bob.ID {
		t.Fatalf("session lookup = %q, %v, %v", userID, ok, err)
	}
	stale, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSession(ctx, bob.ID, stale, now.Add(-time.Hour), "203.0.113.7", "test-agent"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repository.FindSession(ctx, stale.Hash, now); err != nil || ok {
		t.Fatalf("expired session lookup ok = %v, err = %v; want false, nil", ok, err)
	}
	if err := repository.RevokeSession(ctx, token.Hash); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repository.FindSession(ctx, token.Hash, now); err != nil || ok {
		t.Fatalf("revoked session lookup ok = %v, err = %v; want false, nil", ok, err)
	}

	// HTTP login round-trip with the real store, including the
	// unknown-email/wrong-password indistinguishability.
	quote := func(s string) string { return `"` + s + `"` }
	login := func(email, password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
			strings.NewReader(`{"email":`+quote(email)+`,"password":`+quote(password)+`}`)))
		return recorder
	}
	good := login("bob@example.com", "correct-horse-12")
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `"token":"dps_`) {
		t.Fatalf("login status=%d body=%s", good.Code, good.Body.String())
	}
	wrong := login("bob@example.com", "wrong-password-1")
	unknown := login("nobody@example.com", "wrong-password-1")
	if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
		t.Fatalf("statuses = %d/%d, want 401/401", wrong.Code, unknown.Code)
	}
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("responses differ: %q vs %q", wrong.Body.String(), unknown.Body.String())
	}
}

// TestWorkspaceRenameDelete exercises the workspace CRUD paths against
// Postgres: duplicate names (per user, case-insensitive), rename, and
// cascade delete wiping projects, keys, and memberships.
func TestWorkspaceRenameDelete(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)

	var userIDs, workspaceIDs []string
	defer func() {
		for _, id := range workspaceIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, id)
		}
		for _, id := range userIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		}
	}()

	dave, err := repository.CreateUser(ctx, "dave@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, dave.ID)

	first, err := repository.CreateWorkspace(ctx, dave.ID, "Dave Co")
	if err != nil {
		t.Fatal(err)
	}
	workspaceIDs = append(workspaceIDs, first.WorkspaceID)

	// Duplicate create (case-insensitive) is rejected for the same user.
	if _, err := repository.CreateWorkspace(ctx, dave.ID, "dave co"); err != ErrNameTaken {
		t.Fatalf("duplicate create err = %v, want ErrNameTaken", err)
	}

	second, err := repository.CreateWorkspace(ctx, dave.ID, "Side Project")
	if err != nil {
		t.Fatal(err)
	}
	workspaceIDs = append(workspaceIDs, second.WorkspaceID)

	// Rename onto the sibling's name is rejected; a free name succeeds.
	if _, err := repository.RenameWorkspace(ctx, dave.ID, second.WorkspaceID, "DAVE CO"); err != ErrNameTaken {
		t.Fatalf("duplicate rename err = %v, want ErrNameTaken", err)
	}
	renamed, err := repository.RenameWorkspace(ctx, dave.ID, second.WorkspaceID, "Renamed Co")
	if err != nil {
		t.Fatal(err)
	}
	if renamed != "Renamed Co" {
		t.Fatalf("renamed = %q, want %q", renamed, "Renamed Co")
	}

	// A stranger may reuse the same name: uniqueness is per user.
	erin, err := repository.CreateUser(ctx, "erin@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, erin.ID)
	erinWorkspace, err := repository.CreateWorkspace(ctx, erin.ID, "Dave Co")
	if err != nil {
		t.Fatal(err)
	}
	workspaceIDs = append(workspaceIDs, erinWorkspace.WorkspaceID)

	// Seed an API key on the workspace being deleted, then delete: the
	// cascade must wipe members, keys, and projects in one statement.
	key, err := auth.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace_api_keys (workspace_id, name, key_prefix, key_hash, role)
		 VALUES ($1, 'doomed', $2, $3, 'admin')`,
		second.WorkspaceID, key.Prefix, key.Hash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO analytics_projects (workspace_id, name, tracking_id, allowed_domains, timezone, retention_days)
		 VALUES ($1, 'Doomed Site', 'dp_doomed', '{}', 'UTC', 90)`,
		second.WorkspaceID); err != nil {
		t.Fatal(err)
	}

	if err := repository.DeleteWorkspace(ctx, dave.ID, second.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
		   SELECT id FROM workspaces WHERE id = $1
		   UNION ALL SELECT workspace_id FROM workspace_members WHERE workspace_id = $1
		   UNION ALL SELECT workspace_id FROM workspace_api_keys WHERE workspace_id = $1
		   UNION ALL SELECT workspace_id FROM analytics_projects WHERE workspace_id = $1
		 ) doomed`, second.WorkspaceID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("cascade left %d rows for deleted workspace", remaining)
	}
	// Non-members cannot delete or rename the survivor.
	if err := repository.DeleteWorkspace(ctx, erin.ID, first.WorkspaceID); err != ErrNotFound {
		t.Fatalf("stranger delete err = %v, want ErrNotFound", err)
	}
	if _, err := repository.RenameWorkspace(ctx, erin.ID, first.WorkspaceID, "Hijacked"); err != ErrNotFound {
		t.Fatalf("stranger rename err = %v, want ErrNotFound", err)
	}

	// Listing stays deterministic: same order across repeated reads.
	firstList, err := repository.ListWorkspaces(ctx, dave.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondList, err := repository.ListWorkspaces(ctx, dave.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstList) != len(secondList) {
		t.Fatalf("list lengths differ: %d vs %d", len(firstList), len(secondList))
	}
	for i := range firstList {
		if firstList[i] != secondList[i] {
			t.Fatalf("list order unstable: %+v vs %+v", firstList, secondList)
		}
	}
}

// TestTouchSessionSlidesExpiry covers sliding renewal against Postgres:
// an active session is extended at most once per renewalInterval, while
// revoked sessions keep their original expiry for retention.
func TestTouchSessionSlidesExpiry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)

	user, err := repository.CreateUser(ctx, "touch@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	expiresAt := func(hash string) time.Time {
		t.Helper()
		var expiry time.Time
		if err := pool.QueryRow(ctx,
			`SELECT expires_at FROM user_sessions WHERE token_hash = $1`, hash).Scan(&expiry); err != nil {
			t.Fatalf("read expires_at: %v", err)
		}
		return expiry
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	token, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSession(ctx, user.ID, token, now.Add(SessionLifetime), "203.0.113.7", "test-agent"); err != nil {
		t.Fatal(err)
	}
	// A touch within renewalInterval of the stored expiry must not write
	// (throttled): the expiry stays at now + SessionLifetime.
	soon := now.Add(renewalInterval / 2)
	if err := repository.TouchSession(ctx, token.Hash, soon); err != nil {
		t.Fatal(err)
	}
	if got := expiresAt(token.Hash); !got.Equal(now.Add(SessionLifetime)) {
		t.Fatalf("throttled touch changed expiry to %v, want %v", got, now.Add(SessionLifetime))
	}

	// Once renewalInterval has passed the same touch extends the session.
	later := now.Add(renewalInterval)
	if err := repository.TouchSession(ctx, token.Hash, later); err != nil {
		t.Fatal(err)
	}
	if got := expiresAt(token.Hash); !got.Equal(later.Add(SessionLifetime)) {
		t.Fatalf("expiry = %v, want %v", got, later.Add(SessionLifetime))
	}

	// Revoked sessions are never extended.
	if err := repository.RevokeSession(ctx, token.Hash); err != nil {
		t.Fatal(err)
	}
	revokedExpiry := expiresAt(token.Hash)
	if err := repository.TouchSession(ctx, token.Hash, later.Add(2*renewalInterval)); err != nil {
		t.Fatal(err)
	}
	if got := expiresAt(token.Hash); !got.Equal(revokedExpiry) {
		t.Fatalf("revoked expiry = %v, want unchanged %v", got, revokedExpiry)
	}
}

// TestEmailVerificationFlow exercises the full signup gate against
// Postgres: register issues no session, login is refused until the
// emailed link is consumed, resending replaces the old link, and links
// are single-use.
func TestEmailVerificationFlow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	sender := &recordingSender{}
	handler := NewHandler(repository, WithEmailSender(sender), WithAppURL("https://app.example.com"))

	const address = "verify-flow@example.com"
	var userID string
	defer func() {
		if userID == "" {
			return
		}
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM workspaces WHERE id IN (SELECT workspace_id FROM workspace_members WHERE user_id = $1)`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	}()

	login := func(password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
			strings.NewReader(`{"email":"`+address+`","password":"`+password+`"}`)))
		return recorder
	}
	verify := func(token string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.VerifyEmail(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/verify-email",
			strings.NewReader(`{"token":"`+token+`"}`)))
		return recorder
	}

	register := httptest.NewRecorder()
	handler.Register(register, httptest.NewRequest(http.MethodPost, "/v1/auth/register",
		strings.NewReader(`{"email":"`+address+`","password":"correct-horse-12"}`)))
	if register.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", register.Code, register.Body.String())
	}
	if strings.Contains(register.Body.String(), `"token"`) {
		t.Fatalf("register must not issue a session: %s", register.Body.String())
	}
	if len(register.Result().Cookies()) != 0 {
		t.Fatal("register must not set cookies")
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, address).Scan(&userID); err != nil {
		t.Fatalf("read created user: %v", err)
	}

	if got := login("correct-horse-12"); got.Code != http.StatusForbidden {
		t.Fatalf("login before verify: status=%d body=%s, want 403", got.Code, got.Body.String())
	}

	first := tokenFromMessage(t, sender.wait(t, address))

	// Resending replaces the pending link: the original must die.
	resend := httptest.NewRecorder()
	handler.ResendVerification(resend, httptest.NewRequest(http.MethodPost, "/v1/auth/resend-verification",
		strings.NewReader(`{"email":"`+address+`"}`)))
	if resend.Code != http.StatusAccepted {
		t.Fatalf("resend: status=%d body=%s", resend.Code, resend.Body.String())
	}
	if got := verify(first); got.Code != http.StatusBadRequest {
		t.Fatalf("replaced link: status=%d, want 400", got.Code)
	}

	messages := sender.await(t, address, 2)
	second := tokenFromMessage(t, messages[1])
	if got := verify(second); got.Code != http.StatusOK {
		t.Fatalf("verify: status=%d body=%s", got.Code, got.Body.String())
	}
	if got := verify(second); got.Code != http.StatusBadRequest {
		t.Fatalf("link reuse: status=%d, want 400", got.Code)
	}

	verifiedLogin := login("correct-horse-12")
	if verifiedLogin.Code != http.StatusOK || !strings.Contains(verifiedLogin.Body.String(), `"token":"dps_`) {
		t.Fatalf("login after verify: status=%d body=%s", verifiedLogin.Code, verifiedLogin.Body.String())
	}
}

// TestPasswordResetFlow covers forgot/reset against Postgres: the link
// rotates the password, verifies the email (inbox control is the same
// evidence), revokes every existing session, and works exactly once.
func TestPasswordResetFlow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	sender := &recordingSender{}
	handler := NewHandler(repository, WithEmailSender(sender), WithAppURL("https://app.example.com"))

	const address = "reset-flow@example.com"
	user, err := repository.CreateUser(ctx, address, hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	var sessionHash string
	defer func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM workspaces WHERE id IN (SELECT workspace_id FROM workspace_members WHERE user_id = $1)`, user.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()
	membership, err := repository.CreateWorkspace(ctx, user.ID, "Reset Co")
	if err != nil {
		t.Fatal(err)
	}
	_ = membership

	// A live session that the reset must kill.
	existing, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	sessionHash = existing.Hash
	if err := repository.CreateSession(ctx, user.ID, existing, time.Now().UTC().Add(time.Hour), "203.0.113.7", "test-agent"); err != nil {
		t.Fatal(err)
	}

	forgot := httptest.NewRecorder()
	handler.ForgotPassword(forgot, httptest.NewRequest(http.MethodPost, "/v1/auth/forgot-password",
		strings.NewReader(`{"email":"`+address+`"}`)))
	if forgot.Code != http.StatusAccepted {
		t.Fatalf("forgot: status=%d body=%s", forgot.Code, forgot.Body.String())
	}
	raw := tokenFromMessage(t, sender.wait(t, address))

	reset := func(token, password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ResetPassword(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/reset-password",
			strings.NewReader(`{"token":"`+token+`","password":"`+password+`"}`)))
		return recorder
	}
	good := reset(raw, "brand-new-password-42")
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `"password_updated":true`) {
		t.Fatalf("reset: status=%d body=%s", good.Code, good.Body.String())
	}

	if _, ok, err := repository.FindSession(ctx, sessionHash, time.Now().UTC()); err != nil || ok {
		t.Fatalf("pre-reset session still valid: ok=%v err=%v; want revoked", ok, err)
	}

	login := func(password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
			strings.NewReader(`{"email":"`+address+`","password":"`+password+`"}`)))
		return recorder
	}
	if got := login("correct-horse-12"); got.Code != http.StatusUnauthorized {
		t.Fatalf("old password: status=%d, want 401", got.Code)
	}
	// The reset also verified the account: login works without a
	// separate verification step.
	if got := login("brand-new-password-42"); got.Code != http.StatusOK {
		t.Fatalf("new password: status=%d body=%s", got.Code, got.Body.String())
	}

	if got := reset(raw, "another-password-99"); got.Code != http.StatusBadRequest {
		t.Fatalf("link reuse: status=%d, want 400", got.Code)
	}
}

// TestOAuthFlow exercises the GitHub round-trip against Postgres: start
// stores a hashed state, the callback consumes it, creates a verified
// account with its own workspace, links the identity, and sets the
// session cookie. A repeat login reuses the account, a password account
// with the same email is adopted instead of duplicated, and expired or
// replayed states are refused.
func TestOAuthFlow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	provider := &stubProvider{name: "github", identity: oauth.Identity{
		ProviderUserID: "gh-oauth-1",
		Email:          "oauth-flow@example.com",
		EmailVerified:  true,
	}}
	handler := NewHandler(repository, WithOAuthProviders(map[string]oauth.Provider{"github": provider}))

	var createdUserIDs []string
	defer func() {
		for _, id := range createdUserIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM workspaces WHERE id IN (SELECT workspace_id FROM workspace_members WHERE user_id = $1)`, id)
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
		}
	}()

	start := func() string {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/auth/oauth/github/start?next=/projects", nil)
		request.SetPathValue("provider", "github")
		handler.OAuthStart(recorder, request)
		if recorder.Code != http.StatusFound {
			t.Fatalf("start: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		location, err := recorder.Result().Location()
		if err != nil {
			t.Fatal(err)
		}
		return location.Query().Get("state")
	}
	callback := func(state, code string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet,
			"/v1/auth/oauth/github/callback?state="+url.QueryEscape(state)+"&code="+code, nil)
		request.SetPathValue("provider", "github")
		handler.OAuthCallback(recorder, request)
		return recorder
	}

	// First login: account, workspace, binding, session cookie.
	first := callback(start(), "authcode")
	if first.Code != http.StatusFound {
		t.Fatalf("callback: status=%d body=%s", first.Code, first.Body.String())
	}
	if location, _ := first.Result().Location(); location == nil || location.Path != "/projects" {
		t.Fatalf("callback redirect = %v, want /projects", location)
	}
	if cookie := first.Header().Get("Set-Cookie"); !strings.Contains(cookie, sessionCookieName+"=") {
		t.Fatalf("Set-Cookie = %q, want session cookie", cookie)
	}

	var createdID string
	var verified bool
	if err := pool.QueryRow(ctx,
		`SELECT id, email_verified_at IS NOT NULL FROM users WHERE email = 'oauth-flow@example.com'`,
	).Scan(&createdID, &verified); err != nil {
		t.Fatalf("created user: %v", err)
	}
	createdUserIDs = append(createdUserIDs, createdID)
	if !verified {
		t.Fatal("oauth-created account must be email-verified")
	}
	var memberships, bindings int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace_members WHERE user_id = $1`, createdID).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 {
		t.Fatalf("memberships = %d, want 1 personal workspace", memberships)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oauth_accounts
		  WHERE provider = 'github' AND provider_user_id = 'gh-oauth-1' AND user_id = $1`,
		createdID).Scan(&bindings); err != nil {
		t.Fatal(err)
	}
	if bindings != 1 {
		t.Fatalf("bindings = %d, want 1", bindings)
	}

	// States are single-use; missing or empty states are refused.
	if replay := callback("never-issued-state", "x"); replay.Code != http.StatusBadRequest {
		t.Fatalf("bogus state: status=%d, want 400", replay.Code)
	}
	if reuse := callback("", "x"); reuse.Code != http.StatusBadRequest {
		t.Fatalf("empty state: status=%d, want 400", reuse.Code)
	}

	// Second login reuses the account: still exactly one user.
	second := callback(start(), "authcode2")
	if second.Code != http.StatusFound {
		t.Fatalf("second callback: status=%d body=%s", second.Code, second.Body.String())
	}
	var userCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE email = 'oauth-flow@example.com'`).Scan(&userCount); err != nil {
		t.Fatal(err)
	}
	if userCount != 1 {
		t.Fatalf("user count = %d, want 1", userCount)
	}

	// Adopting an existing password account: link + verify, no duplicate.
	adoptEmail := "adopt-flow@example.com"
	adopted, err := repository.CreateUser(ctx, adoptEmail, hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	createdUserIDs = append(createdUserIDs, adopted.ID)
	provider.identity = oauth.Identity{ProviderUserID: "gh-oauth-2", Email: adoptEmail, EmailVerified: true}
	if adopt := callback(start(), "adoptcode"); adopt.Code != http.StatusFound {
		t.Fatalf("adopt callback: status=%d body=%s", adopt.Code, adopt.Body.String())
	}
	var adoptedID string
	if err := pool.QueryRow(ctx,
		`SELECT user_id::text FROM oauth_accounts WHERE provider = 'github' AND provider_user_id = 'gh-oauth-2'`,
	).Scan(&adoptedID); err != nil {
		t.Fatalf("adopted binding: %v", err)
	}
	if adoptedID != adopted.ID {
		t.Fatalf("adopted binding user = %s, want %s", adoptedID, adopted.ID)
	}
	login := httptest.NewRecorder()
	handler.Login(login, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"`+adoptEmail+`","password":"correct-horse-12"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("password login after adoption: status=%d body=%s; want verified account", login.Code, login.Body.String())
	}

	// Expired states are refused.
	expiredRaw, expiredHash, err := oauth.GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateOAuthState(ctx, "github", expiredHash, "verifier", "/",
		time.Now().UTC().Add(-2*time.Hour), time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if expired := callback(expiredRaw, "c"); expired.Code != http.StatusBadRequest {
		t.Fatalf("expired state: status=%d, want 400", expired.Code)
	}
}

// TestPhase4SessionsLockoutTOTP exercises the three Phase-4 features
// against the real store: session metadata + revocation, the failed-
// login lockout, and the TOTP enrollment/login gate.
func TestPhase4SessionsLockoutTOTP(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	handler := NewHandler(repository)

	var createdUserID string
	defer func() {
		if createdUserID == "" {
			return
		}
		_, _ = pool.Exec(ctx, `DELETE FROM workspaces WHERE id IN (SELECT workspace_id FROM workspace_members WHERE user_id = $1)`, createdUserID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, createdUserID)
	}()

	user, err := repository.CreateUser(ctx, "phase4@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	createdUserID = user.ID
	if _, err := repository.CreateWorkspace(ctx, user.ID, "Phase4"); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetEmailVerified(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	login := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
		request.Header.Set("User-Agent", "phase4-integration/1.0")
		request.RemoteAddr = "198.51.100.9:4242"
		handler.Login(recorder, request)
		return recorder
	}
	// Session-guarded endpoints sit behind RequireSession in main.go;
	// replay that middleware here so the Bearer token is what (only)
	// authenticates the call.
	guarded := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth.RequireSession(repository, time.Now, h).ServeHTTP(w, r)
		}
	}

	// --- Sessions: two logins store ip/ua; list marks the caller's own.
	first := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("login1: status=%d body=%s", first.Code, first.Body.String())
	}
	second := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`)
	if second.Code != http.StatusOK {
		t.Fatalf("login2: status=%d body=%s", second.Code, second.Body.String())
	}
	var rawToken string
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	rawToken = body.Token
	listRequest := httptest.NewRequest(http.MethodGet, "/v1/auth/sessions", nil)
	listRequest.Header.Set("Authorization", "Bearer "+rawToken)
	listRecorder := httptest.NewRecorder()
	guarded(handler.ListSessions)(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("sessions list: status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var sessions []SessionInfo
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2 (%s)", len(sessions), listRecorder.Body.String())
	}
	currentCount := 0
	for _, session := range sessions {
		if session.IP != "198.51.100.9" || session.UserAgent != "phase4-integration/1.0" {
			t.Fatalf("session metadata = %q/%q, want ip/ua recorded", session.IP, session.UserAgent)
		}
		if session.Current {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("current sessions = %d, want exactly 1", currentCount)
	}

	// Revoke the session that is NOT current; the current one survives.
	var otherID string
	for _, session := range sessions {
		if !session.Current {
			otherID = session.ID
		}
	}
	revokeRequest := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions/"+otherID, nil)
	revokeRequest.Header.Set("Authorization", "Bearer "+rawToken)
	revokeRequest.SetPathValue("id", otherID)
	revokeRecorder := httptest.NewRecorder()
	guarded(handler.RevokeSession)(revokeRecorder, revokeRequest)
	if revokeRecorder.Code != http.StatusNoContent {
		t.Fatalf("revoke: status=%d body=%s", revokeRecorder.Code, revokeRecorder.Body.String())
	}
	listRecorder = httptest.NewRecorder()
	guarded(handler.ListSessions)(listRecorder, listRequest)
	sessions = nil
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || !sessions[0].Current {
		t.Fatalf("after revoke sessions = %+v, want just the current one", sessions)
	}

	// Another user's session ID → 404, no cross-user revocation.
	otherUser, err := repository.CreateUser(ctx, "phase4-other@example.com", hashForTest(t, "correct-horse-12"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUser.ID)
	}()
	victimToken, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSession(ctx, otherUser.ID, victimToken, time.Now().UTC().Add(time.Hour), "203.0.113.1", "x"); err != nil {
		t.Fatal(err)
	}
	var victimSessionID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM user_sessions WHERE token_hash = $1`, victimToken.Hash).Scan(&victimSessionID); err != nil {
		t.Fatal(err)
	}
	foreign := httptest.NewRequest(http.MethodDelete, "/v1/auth/sessions/"+victimSessionID, nil)
	foreign.Header.Set("Authorization", "Bearer "+rawToken)
	foreign.SetPathValue("id", victimSessionID)
	foreignRecorder := httptest.NewRecorder()
	guarded(handler.RevokeSession)(foreignRecorder, foreign)
	if foreignRecorder.Code != http.StatusNotFound {
		t.Fatalf("foreign revoke: status=%d, want 404", foreignRecorder.Code)
	}
	// Their session is untouched.
	if userID, ok, err := repository.FindSession(ctx, victimToken.Hash, time.Now().UTC()); err != nil || !ok || userID != otherUser.ID {
		t.Fatalf("victim session after foreign revoke = %q/%v/%v", userID, ok, err)
	}

	// --- Lockout: five wrong passwords lock, correct one still 429.
	for range loginLockoutThreshold {
		if attempt := login(`{"email":"phase4@example.com","password":"wrong-password-1"}`); attempt.Code != http.StatusUnauthorized && attempt.Code != http.StatusTooManyRequests {
			t.Fatalf("wrong password: status=%d", attempt.Code)
		}
	}
	locked := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`)
	if locked.Code != http.StatusTooManyRequests || locked.Header().Get("Retry-After") == "" {
		t.Fatalf("locked login: status=%d retry-after=%q body=%s", locked.Code, locked.Header().Get("Retry-After"), locked.Body.String())
	}
	// Clear the lock and confirm the account works again.
	if _, err := pool.Exec(ctx, `UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if recovered := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`); recovered.Code != http.StatusOK {
		t.Fatalf("after unlock: status=%d body=%s", recovered.Code, recovered.Body.String())
	}

	// --- TOTP: setup → enable → password-only login refused → code works.
	setupRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/2fa/setup", strings.NewReader("{}"))
	setupRequest.Header.Set("Authorization", "Bearer "+rawToken)
	setupRecorder := httptest.NewRecorder()
	guarded(handler.TwoFactorSetup)(setupRecorder, setupRequest)
	if setupRecorder.Code != http.StatusOK {
		t.Fatalf("2fa setup: status=%d body=%s", setupRecorder.Code, setupRecorder.Body.String())
	}
	var setup struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(setupRecorder.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	enableRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/2fa/enable", strings.NewReader(`{"code":"`+code+`"}`))
	enableRequest.Header.Set("Authorization", "Bearer "+rawToken)
	enableRecorder := httptest.NewRecorder()
	guarded(handler.TwoFactorEnable)(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("2fa enable: status=%d body=%s", enableRecorder.Code, enableRecorder.Body.String())
	}

	// Password alone now fails; the live code passes.
	noCode := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`)
	if noCode.Code != http.StatusUnauthorized || !strings.Contains(noCode.Body.String(), "two-factor code required") {
		t.Fatalf("2fa gate: status=%d body=%s", noCode.Code, noCode.Body.String())
	}
	newCode, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	withCode := login(`{"email":"phase4@example.com","password":"correct-horse-12","totp_code":"` + newCode + `"}`)
	if withCode.Code != http.StatusOK {
		t.Fatalf("2fa login: status=%d body=%s", withCode.Code, withCode.Body.String())
	}

	// Disable with the live code, then password-only works again.
	code, err = totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	disableRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/2fa/disable", strings.NewReader(`{"code":"`+code+`"}`))
	disableRequest.Header.Set("Authorization", "Bearer "+rawToken)
	disableRecorder := httptest.NewRecorder()
	guarded(handler.TwoFactorDisable)(disableRecorder, disableRequest)
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("2fa disable: status=%d body=%s", disableRecorder.Code, disableRecorder.Body.String())
	}
	if plain := login(`{"email":"phase4@example.com","password":"correct-horse-12"}`); plain.Code != http.StatusOK {
		t.Fatalf("post-disable login: status=%d body=%s", plain.Code, plain.Body.String())
	}
}
