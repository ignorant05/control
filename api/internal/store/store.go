package store

import (
	"context"

	"github.com/ignorant05/control/api/cmd/model"
)

// Store defines all persistence operations
type Store interface {
	// Projects
	CreateProject(ctx context.Context, name string) (*model.Project, error)
	GetProject(ctx context.Context, id string) (*model.Project, error)
	GetProjectByName(ctx context.Context, name string) (*model.Project, error)
	ListProjects(ctx context.Context) ([]*model.Project, error)
	DeleteProject(ctx context.Context, id string) error

	// Users
	CreateUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, id string) (*model.User, error)
	GetUserByName(ctx context.Context, name string) (*model.User, error)
	ListUsers(ctx context.Context) ([]*model.User, error)
	ListUsersByProject(ctx context.Context, projectID string) ([]*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	DeleteUser(ctx context.Context, id string) error

	// Feature Flags
	CreateFlag(ctx context.Context, flag *model.FeatureFlag) error
	GetFlag(ctx context.Context, id string) (*model.FeatureFlag, error)
	GetFlagByName(ctx context.Context, projectID, name string) (*model.FeatureFlag, error)
	ListFlags(ctx context.Context, projectID string) ([]*model.FeatureFlag, error)
	UpdateFlag(ctx context.Context, flag *model.FeatureFlag) error
	DeleteFlag(ctx context.Context, id string) error

	// Audit Log
	LogAudit(ctx context.Context, entry *model.AuditEntry) error
	GetAuditLog(ctx context.Context, projectID string, limit int) ([]*model.AuditEntry, error)

	// Close
	Close() error
}
