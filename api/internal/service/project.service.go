package service

import (
	"context"
	"fmt"

	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/store"
)

type ProjectService struct {
	store store.Store
	redis *store.RedisStore
}

func NewProjectService(s store.Store, r *store.RedisStore) *ProjectService {
	return &ProjectService{store: s, redis: r}
}

// GetProject retrieves a project with its flags and admin names
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*model.Project, error) {
	if s.redis != nil {
		var cached model.Project
		if err := s.redis.GetJSON(ctx, fmt.Sprintf("project:%s", projectID), &cached); err == nil {
			return &cached, nil
		}
	}

	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	flags, err := s.store.ListFlags(ctx, projectID)
	if err != nil {
		return nil, err
	}
	project.Flags = flags

	users, err := s.store.ListUsersByProject(ctx, project.Name)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Role == model.RoleAdmin {
			project.AdminNames = append(project.AdminNames, u.Name)
		}
	}

	if s.redis != nil {
		s.redis.SetJSON(ctx, fmt.Sprintf("project:%s", projectID), project, 0)
	}

	return project, nil
}

// GetProjectByName retrieves a project by name
func (s *ProjectService) GetProjectByName(ctx context.Context, name string) (*model.Project, error) {
	return s.store.GetProjectByName(ctx, name)
}

// ListProjects returns all projects with summary stats
func (s *ProjectService) ListProjects(ctx context.Context) ([]*model.Project, error) {
	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range projects {
		flags, err := s.store.ListFlags(ctx, p.ID)
		if err != nil {
			continue
		}
		p.Flags = flags

		users, err := s.store.ListUsersByProject(ctx, p.Name)
		if err != nil {
			continue
		}
		for _, u := range users {
			if u.Role == model.RoleAdmin {
				p.AdminNames = append(p.AdminNames, u.Name)
			}
		}
	}

	return projects, nil
}

// DeleteProject removes a project and all associated data
func (s *ProjectService) DeleteProject(ctx context.Context, projectID string) error {
	if err := s.store.DeleteProject(ctx, projectID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if s.redis != nil {
		s.redis.InvalidateProject(ctx, projectID)
	}
	return nil
}
