package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const projectColumns = `id::text, name, tracking_id, allowed_domains, timezone, retention_days, enabled, created_at, updated_at`

type Repository interface {
	Create(context.Context, CreateInput, string) (*Project, error)
	List(context.Context) ([]Project, error)
	Get(context.Context, string) (*Project, error)
	Update(context.Context, string, UpdateInput) (*Project, error)
	Delete(context.Context, string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Create(ctx context.Context, input CreateInput, trackingID string) (*Project, error) {
	columns := []string{"name", "tracking_id", "allowed_domains"}
	placeholders := []string{"$1", "$2", "$3"}
	args := []any{input.Name, trackingID, input.AllowedDomains}

	if input.Timezone != nil {
		columns = append(columns, "timezone")
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, *input.Timezone)
	}
	if input.RetentionDays != nil {
		columns = append(columns, "retention_days")
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, *input.RetentionDays)
	}

	query := fmt.Sprintf(`INSERT INTO analytics_projects (%s) VALUES (%s) RETURNING %s`, strings.Join(columns, ", "), strings.Join(placeholders, ", "), projectColumns)
	project, err := scanProject(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

func (repository *PostgresRepository) List(ctx context.Context) ([]Project, error) {
	rows, err := repository.pool.Query(ctx, `SELECT `+projectColumns+` FROM analytics_projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, *project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func (repository *PostgresRepository) Get(ctx context.Context, id string) (*Project, error) {
	project, err := scanProject(repository.pool.QueryRow(ctx, `SELECT `+projectColumns+` FROM analytics_projects WHERE id = $1`, id))
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

func (repository *PostgresRepository) Update(ctx context.Context, id string, input UpdateInput) (*Project, error) {
	assignments := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if input.Name != nil {
		args = append(args, *input.Name)
		assignments = append(assignments, fmt.Sprintf("name = $%d", len(args)))
	}
	if input.AllowedDomains != nil {
		args = append(args, *input.AllowedDomains)
		assignments = append(assignments, fmt.Sprintf("allowed_domains = $%d", len(args)))
	}
	if input.Timezone != nil {
		args = append(args, *input.Timezone)
		assignments = append(assignments, fmt.Sprintf("timezone = $%d", len(args)))
	}
	if input.RetentionDays != nil {
		args = append(args, *input.RetentionDays)
		assignments = append(assignments, fmt.Sprintf("retention_days = $%d", len(args)))
	}
	if input.Enabled != nil {
		args = append(args, *input.Enabled)
		assignments = append(assignments, fmt.Sprintf("enabled = $%d", len(args)))
	}
	assignments = append(assignments, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE analytics_projects SET %s WHERE id = $%d RETURNING %s`, strings.Join(assignments, ", "), len(args), projectColumns)
	project, err := scanProject(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return project, nil
}

func (repository *PostgresRepository) Delete(ctx context.Context, id string) error {
	commandTag, err := repository.pool.Exec(ctx, `DELETE FROM analytics_projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanProject(row rowScanner) (*Project, error) {
	project := new(Project)
	err := row.Scan(
		&project.ID,
		&project.Name,
		&project.TrackingID,
		&project.AllowedDomains,
		&project.Timezone,
		&project.RetentionDays,
		&project.Enabled,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err == nil {
		return project, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return nil, ErrConflict
	}
	return nil, err
}
