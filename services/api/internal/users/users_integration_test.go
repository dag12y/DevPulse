package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/database"
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
	if err := repository.CreateSession(ctx, bob.ID, token, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if userID, ok, err := repository.FindSession(ctx, token.Hash, now); err != nil || !ok || userID != bob.ID {
		t.Fatalf("session lookup = %q, %v, %v", userID, ok, err)
	}
	stale, err := auth.GenerateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateSession(ctx, bob.ID, stale, now.Add(-time.Hour)); err != nil {
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
