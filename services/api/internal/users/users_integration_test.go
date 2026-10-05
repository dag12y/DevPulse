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
