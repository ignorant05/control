package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ignorant05/control/api/cmd/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(ctx context.Context, connString string) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	store := &PostgresStore{pool: pool}
	if err := store.migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('Admin', 'Editor', 'Viewer')),
			project TEXT NOT NULL REFERENCES projects(name) ON DELETE CASCADE,
			logged_in BOOLEAN DEFAULT FALSE,
			password TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS feature_flags (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('ON', 'OFF')),
			rollout INTEGER NOT NULL DEFAULT 0 CHECK (rollout >= 0 AND rollout <= 100),
			description TEXT,
			killed BOOLEAN DEFAULT FALSE,
			targeted_users TEXT[] DEFAULT '{}',
			last_modified TIMESTAMPTZ DEFAULT NOW(),
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			UNIQUE(name, project_id)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			user_name TEXT NOT NULL,
			action TEXT NOT NULL,
			flag_name TEXT,
			detail TEXT,
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flags_project ON feature_flags(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_project ON audit_log(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_project ON users(project)`,
	}

	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}

// Projects
func (s *PostgresStore) CreateProject(ctx context.Context, name string) (*model.Project, error) {
	id := uuid.New().String()
	_, err := s.pool.Exec(ctx,
		"INSERT INTO projects (id, name) VALUES ($1, $2)",
		id, name)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	return s.GetProject(ctx, id)
}

func (s *PostgresStore) GetProject(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		"SELECT id, name, created_at FROM projects WHERE id = $1", id).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &p, nil
}

func (s *PostgresStore) GetProjectByName(ctx context.Context, name string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		"SELECT id, name, created_at FROM projects WHERE name = $1", name).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project by name: %w", err)
	}
	return &p, nil
}

func (s *PostgresStore) ListProjects(ctx context.Context) ([]*model.Project, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, name, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, rows.Err()
}

func (s *PostgresStore) DeleteProject(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", id)
	return err
}

// Users
func (s *PostgresStore) CreateUser(ctx context.Context, user *model.User) error {
	user.ID = uuid.New().String()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, name, role, project, logged_in, password)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.Name, string(user.Role), user.Project, user.LoggedIn, user.Password)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetUser(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		"SELECT id, name, role, project, logged_in, password FROM users WHERE id = $1", id).
		Scan(&u.ID, &u.Name, &u.Role, &u.Project, &u.LoggedIn, &u.Password)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (s *PostgresStore) GetUserByName(ctx context.Context, name string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		"SELECT id, name, role, project, logged_in, password FROM users WHERE name = $1", name).
		Scan(&u.ID, &u.Name, &u.Role, &u.Project, &u.LoggedIn, &u.Password)
	if err != nil {
		return nil, fmt.Errorf("get user by name: %w", err)
	}
	return &u, nil
}

func (s *PostgresStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, name, role, project, logged_in, password FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Role, &u.Project, &u.LoggedIn, &u.Password); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (s *PostgresStore) ListUsersByProject(ctx context.Context, projectName string) ([]*model.User, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, name, role, project, logged_in, password FROM users WHERE project = $1 ORDER BY created_at DESC",
		projectName)
	if err != nil {
		return nil, fmt.Errorf("list users by project: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Role, &u.Project, &u.LoggedIn, &u.Password); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (s *PostgresStore) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET name = $1, role = $2, project = $3, logged_in = $4, password = $5 WHERE id = $6`,
		user.Name, string(user.Role), user.Project, user.LoggedIn, user.Password, user.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// Feature Flags
func (s *PostgresStore) CreateFlag(ctx context.Context, flag *model.FeatureFlag) error {
	flag.ID = uuid.New().String()
	flag.LastModified = time.Now()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO feature_flags (id, name, status, rollout, description, killed, targeted_users, last_modified, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		flag.ID, flag.Name, string(flag.Status), flag.Rollout, flag.Description,
		flag.Killed, flag.TargetedUsers, flag.LastModified, flag.ProjectID)
	if err != nil {
		return fmt.Errorf("insert flag: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetFlag(ctx context.Context, id string) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, status, rollout, description, killed, targeted_users, last_modified, project_id
		 FROM feature_flags WHERE id = $1`, id).
		Scan(&f.ID, &f.Name, &f.Status, &f.Rollout, &f.Description, &f.Killed,
			&f.TargetedUsers, &f.LastModified, &f.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get flag: %w", err)
	}
	return &f, nil
}

func (s *PostgresStore) GetFlagByName(ctx context.Context, projectID, name string) (*model.FeatureFlag, error) {
	var f model.FeatureFlag
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, status, rollout, description, killed, targeted_users, last_modified, project_id
		 FROM feature_flags WHERE project_id = $1 AND name = $2`, projectID, name).
		Scan(&f.ID, &f.Name, &f.Status, &f.Rollout, &f.Description, &f.Killed,
			&f.TargetedUsers, &f.LastModified, &f.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get flag by name: %w", err)
	}
	return &f, nil
}

func (s *PostgresStore) ListFlags(ctx context.Context, projectID string) ([]*model.FeatureFlag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, status, rollout, description, killed, targeted_users, last_modified, project_id
		 FROM feature_flags WHERE project_id = $1 AND killed = FALSE ORDER BY last_modified DESC`,
		projectID)
	if err != nil {
		return nil, fmt.Errorf("list flags: %w", err)
	}
	defer rows.Close()

	var flags []*model.FeatureFlag
	for rows.Next() {
		var f model.FeatureFlag
		if err := rows.Scan(&f.ID, &f.Name, &f.Status, &f.Rollout, &f.Description, &f.Killed,
			&f.TargetedUsers, &f.LastModified, &f.ProjectID); err != nil {
			return nil, err
		}
		flags = append(flags, &f)
	}
	return flags, rows.Err()
}

func (s *PostgresStore) UpdateFlag(ctx context.Context, flag *model.FeatureFlag) error {
	flag.LastModified = time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE feature_flags SET name = $1, status = $2, rollout = $3, description = $4,
		 killed = $5, targeted_users = $6, last_modified = $7 WHERE id = $8`,
		flag.Name, string(flag.Status), flag.Rollout, flag.Description,
		flag.Killed, flag.TargetedUsers, flag.LastModified, flag.ID)
	if err != nil {
		return fmt.Errorf("update flag: %w", err)
	}
	return nil
}

func (s *PostgresStore) DeleteFlag(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM feature_flags WHERE id = $1", id)
	return err
}

// Audit Log
func (s *PostgresStore) LogAudit(ctx context.Context, entry *model.AuditEntry) error {
	entry.ID = uuid.New().String()
	entry.Time = time.Now()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (id, created_at, user_name, action, flag_name, detail, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.ID, entry.Time, entry.User, string(entry.Action), entry.Flag, entry.Detail, entry.ProjectID)
	if err != nil {
		return fmt.Errorf("log audit: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetAuditLog(ctx context.Context, projectID string, limit int) ([]*model.AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, created_at, user_name, action, flag_name, detail, project_id
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2`,
		projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("get audit log: %w", err)
	}
	defer rows.Close()

	var entries []*model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.User, &e.Action, &e.Flag, &e.Detail, &e.ProjectID); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}
