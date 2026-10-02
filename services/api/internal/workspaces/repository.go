package workspaces

import (
	"context"
	"fmt"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIKey struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	Prefix      string  `json:"key_prefix"`
	Role        string  `json:"role"`
	RevokedAt   *string `json:"revoked_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Count returns the number of workspaces (used to gate bootstrap).
func (r *Repository) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count workspaces: %w", err)
	}
	return n, nil
}

// CreateWorkspace inserts a workspace and returns it.
func (r *Repository) CreateWorkspace(ctx context.Context, name string) (*Workspace, error) {
	workspace := new(Workspace)
	err := r.pool.QueryRow(ctx,
		`INSERT INTO workspaces (name) VALUES ($1) RETURNING id::text, name`,
		name).Scan(&workspace.ID, &workspace.Name)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return workspace, nil
}

// CreateKey persists a hashed API key. The raw key is never stored.
func (r *Repository) CreateKey(ctx context.Context, workspaceID, name, role string, generated auth.GeneratedKey) (*APIKey, error) {
	key := new(APIKey)
	err := r.pool.QueryRow(ctx,
		`INSERT INTO workspace_api_keys (workspace_id, name, key_prefix, key_hash, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text, workspace_id::text, name, key_prefix, role,
		           revoked_at::text, created_at::text`,
		workspaceID, name, generated.Prefix, generated.Hash, role).Scan(
		&key.ID, &key.WorkspaceID, &key.Name, &key.Prefix, &key.Role, &key.RevokedAt, &key.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}
	return key, nil
}

// FindKey implements auth.KeyFinder. Revoked keys behave as unknown.
func (r *Repository) FindKey(ctx context.Context, keyHash string) (string, string, bool, error) {
	var workspaceID, role string
	err := r.pool.QueryRow(ctx,
		`SELECT workspace_id::text, role FROM workspace_api_keys
		  WHERE key_hash = $1 AND revoked_at IS NULL`,
		keyHash).Scan(&workspaceID, &role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("find API key: %w", err)
	}
	return workspaceID, role, true, nil
}

// RevokeKey soft-revokes a key within its workspace.
func (r *Repository) RevokeKey(ctx context.Context, workspaceID, keyID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspace_api_keys SET revoked_at = NOW()
		  WHERE id = $1 AND workspace_id = $2 AND revoked_at IS NULL`,
		keyID, workspaceID)
	if err != nil {
		return fmt.Errorf("revoke API key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
